package poker

import (
	"bytes"
	"dkforest/pkg/database"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"dkforest/pkg/utils/rwmtx"
	hutils "dkforest/pkg/web/handlers/utils"
	"errors"
	"fmt"
	"github.com/chehsunliu/poker"
	"github.com/sirupsen/logrus"
	"html/template"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const NbPlayers = 6
const MaxUserCountdown = 60
const MinTimeAfterGame = 10
const BackfacingDeg = "-180deg"
const BurnStackX = 400
const BurnStackY = 30
const DealX = 155
const DealY = 130
const DealSpacing = 55
const DealerStackX = 250
const DealerStackY = 30
const NbCardsPerPlayer = 2
const animationTime = 1000 * time.Millisecond
const RakeBackPct = 0.30

type Poker struct {
	sync.Mutex
	games map[RoomID]*Game
}

func newPoker() *Poker {
	p := &Poker{}
	p.games = make(map[RoomID]*Game)
	return p
}

func (p *Poker) GetGame(roomID RoomID) *Game {
	p.Lock()
	defer p.Unlock()
	g, found := PokerInstance.games[roomID]
	if !found {
		return nil
	}
	return g
}

func (p *Poker) GetOrCreateGame(db *database.DkfDB, roomID RoomID, pokerTableID int64,
	pokerTableMinBet database.PokerChip, pokerTableIsTest bool) *Game {
	p.Lock()
	defer p.Unlock()
	g, found := p.games[roomID]
	if !found {
		g = p.createGame(db, roomID, pokerTableID, pokerTableMinBet, pokerTableIsTest)
	}
	return g
}

func (p *Poker) CreateGame(db *database.DkfDB, roomID RoomID, pokerTableID int64,
	pokerTableMinBet database.PokerChip, pokerTableIsTest bool) *Game {
	p.Lock()
	defer p.Unlock()
	return p.createGame(db, roomID, pokerTableID, pokerTableMinBet, pokerTableIsTest)
}

func (p *Poker) createGame(db *database.DkfDB, roomID RoomID, pokerTableID int64,
	pokerTableMinBet database.PokerChip, pokerTableIsTest bool) *Game {
	g := p.newGame(db, roomID, pokerTableID, pokerTableMinBet, pokerTableIsTest)
	p.games[roomID] = g
	return g
}

func (p *Poker) newGame(db *database.DkfDB, roomID RoomID, pokerTableID int64,
	pokerTableMinBet database.PokerChip, pokerTableIsTest bool) *Game {
	g := &Game{
		db:               db,
		roomID:           roomID,
		pokerTableID:     pokerTableID,
		tableType:        TableTypeRake,
		pokerTableMinBet: pokerTableMinBet,
		pokerTableIsTest: pokerTableIsTest,
		playersEventCh:   make(chan playerEvent),
		Players:          rwmtx.New(make(seatedPlayers, NbPlayers)),
		seatsAnimation:   make([]bool, NbPlayers),
		dealerSeatIdx:    atomic.Int32{},
	}
	g.dealerSeatIdx.Store(-1)
	return g
}

type playerEvent struct {
	UserID database.UserID
	Call   bool
	Check  bool
	Fold   bool
	AllIn  bool
	Unsit  bool
	Raise  bool
	Bet    database.PokerChip
}

func (e playerEvent) getAction() PlayerAction {
	action := NoAction
	if e.Fold {
		action = FoldAction
	} else if e.Call {
		action = CallAction
	} else if e.Check {
		action = CheckAction
	} else if e.Bet > 0 {
		action = BetAction
	} else if e.Raise {
		action = RaiseAction
	} else if e.AllIn {
		action = AllInAction
	}
	return action
}

var PokerInstance = newPoker()

type ongoingGame struct {
	logEvents       rwmtx.RWMtxSlice[LogEvent]
	events          rwmtx.RWMtxSlice[PokerEvent]
	waitTurnEvent   rwmtx.RWMtx[PokerWaitTurnEvent]
	autoActionEvent rwmtx.RWMtx[AutoActionEvent]
	mainPot         rwmtx.RWMtx[database.PokerChip]
	minBet          rwmtx.RWMtx[database.PokerChip]
	minRaise        rwmtx.RWMtx[database.PokerChip]
	playerToPlay    rwmtx.RWMtx[database.UserID]
	hasBet          rwmtx.RWMtx[bool]
	players         pokerPlayers
	createdAt       time.Time
	communityCards  []string
	deck            []string
}

type pokerPlayers []*PokerPlayer

type seatedPlayers []*seatedPlayer

func (p pokerPlayers) get(userID database.UserID) *PokerPlayer {
	for _, player := range p {
		if player != nil && player.userID == userID {
			return player
		}
	}
	return nil
}

func (p seatedPlayers) get(userID database.UserID) (out *seatedPlayer) {
	for _, player := range p {
		if player != nil && player.userID == userID {
			return player
		}
	}
	return
}

func (p seatedPlayers) resetStatuses() {
	for _, player := range p {
		player.status.Set("")
	}
}

func (p seatedPlayers) toPokerPlayers() pokerPlayers {
	players := make([]*PokerPlayer, 0)
	for _, player := range p {
		players = append(players, &PokerPlayer{seatedPlayer: player})
	}
	return players
}

func (g *Game) getEligibles() (out seatedPlayers) {
	eligiblePlayers := make(seatedPlayers, 0)
	g.Players.RWith(func(gPlayers seatedPlayers) {
		for _, p := range gPlayers {
			if p.isEligible(g.pokerTableMinBet) {
				eligiblePlayers = append(eligiblePlayers, p)
			}
		}
	})
	return eligiblePlayers
}

type seatedPlayer struct {
	seatIdx         int
	userID          database.UserID
	username        database.Username
	cash            rwmtx.RWMtxUInt64[database.PokerChip]
	status          rwmtx.RWMtx[string]
	hasChecked      bool
	lastActionTS    time.Time
	pokerReferredBy *database.UserID
}

func (p *seatedPlayer) getCash() (out database.PokerChip) {
	return p.cash.Get()
}

func (p *seatedPlayer) getStatus() (out string) {
	return p.status.Get()
}

// Return either or not a player is eligible to play a game
func (p *seatedPlayer) isEligible(pokerTableMinBet database.PokerChip) bool {
	return p != nil && p.getCash() >= pokerTableMinBet
}

type PokerPlayer struct {
	*seatedPlayer
	bet                  rwmtx.RWMtxUInt64[database.PokerChip]
	cards                rwmtx.RWMtxSlice[playerCard]
	folded               atomic.Bool
	unsit                atomic.Bool
	gameBet              database.PokerChip
	allInMaxGain         database.PokerChip
	rakePaid             float64
	countChancesToAction int
}

func (p *PokerPlayer) maxGain(mainPot database.PokerChip) database.PokerChip {
	m := utils.MinInt(p.allInMaxGain, mainPot)
	return utils.Ternary(p.isAllIn(), m, mainPot)
}

func (g *Game) IsBet() (out bool) {
	if g.ongoing != nil {
		return !g.ongoing.hasBet.Get()
	}
	return
}

func (g *Game) IsYourTurn(player *PokerPlayer) (out bool) {
	if g.ongoing != nil {
		return player.userID == g.ongoing.playerToPlay.Get()
	}
	return
}

func (g *Game) CanCheck(player *PokerPlayer) (out bool) {
	if g.ongoing != nil {
		return player.bet.Get() == g.ongoing.minBet.Get()
	}
	return
}

func (g *Game) CanFold(player *PokerPlayer) (out bool) {
	if g.ongoing != nil {
		return player.bet.Get() < g.ongoing.minBet.Get()
	}
	return
}

func (g *Game) MinBet() (out database.PokerChip) {
	if g.ongoing != nil {
		return g.ongoing.minBet.Get()
	}
	return
}

func (p *Game) MinRaise() (out database.PokerChip) {
	if p.ongoing != nil {
		return p.ongoing.minRaise.Get()
	}
	return
}

func (p *PokerPlayer) GetBet() (out database.PokerChip) {
	return p.bet.Get()
}

func (p *PokerPlayer) canBet() bool {
	return !p.folded.Load() && !p.isAllIn()
}

func (p *PokerPlayer) isAllIn() bool {
	return p.getCash() == 0
}

func (p *PokerPlayer) refundPartialBet(db *database.DkfDB, pokerTableID int64, diff database.PokerChip) {
	_ = db.PokerTableAccountRefundPartialBet(p.userID, pokerTableID, diff)
	p.tmp(-diff)
}

func (p *PokerPlayer) doBet(db *database.DkfDB, pokerTableID int64, bet database.PokerChip) {
	_ = db.PokerTableAccountBet(p.userID, pokerTableID, bet)
	p.tmp(bet)
}

func (p *PokerPlayer) tmp(diff database.PokerChip) {
	p.gameBet += diff
	p.bet.Incr(diff)
	p.cash.Incr(-diff)
}

func (p *PokerPlayer) gain(db *database.DkfDB, pokerTableID int64, gain database.PokerChip) {
	_ = db.PokerTableAccountGain(p.userID, pokerTableID, gain)
	p.cash.Incr(gain)
	p.bet.Set(0)
}

// Reset player's bet to 0 and return the value it had before the reset
func (p *PokerPlayer) resetBet() (old database.PokerChip) {
	// Do not track in database
	// DB keeps track of what was bet during the whole (1 hand) game
	return p.bet.Replace(0)
}

func (p *PokerPlayer) refundBet(db *database.DkfDB, pokerTableID int64) {
	p.gain(db, pokerTableID, p.GetBet())
}

func (p *PokerPlayer) doBetAndNotif(g *Game, bet database.PokerChip) {
	p.doBet(g.db, g.pokerTableID, bet)
	PubSub.Pub(g.roomID.Topic(), PlayerBetEvent{PlayerSeatIdx: p.seatIdx, Player: p.username, Bet: bet, TotalBet: p.GetBet(), Cash: p.getCash()})
	pubCashBonus(g, p.seatIdx, bet, false)
}

func pubCashBonus(g *Game, seatIdx int, amount database.PokerChip, isGain bool) {
	g.seatsAnimation[seatIdx] = !g.seatsAnimation[seatIdx]
	PubSub.Pub(g.roomID.Topic(), CashBonusEvent{PlayerSeatIdx: seatIdx, Amount: amount, Animation: g.seatsAnimation[seatIdx], IsGain: isGain})
}

type playerCard struct {
	idx  int
	zIdx int
	name string
}

type Game struct {
	Players          rwmtx.RWMtx[seatedPlayers]
	seatsAnimation   []bool
	ongoing          *ongoingGame
	db               *database.DkfDB
	roomID           RoomID
	pokerTableID     int64
	tableType        int
	pokerTableMinBet database.PokerChip
	pokerTableIsTest bool
	playersEventCh   chan playerEvent
	dealerSeatIdx    atomic.Int32
	isGameStarted    atomic.Bool
}

type gameResult struct {
	handScore int32
	players   []*PokerPlayer
}

func (g *Game) GetLogs() (out []LogEvent) {
	if g.ongoing != nil {
		out = g.ongoing.logEvents.Clone()
	}
	return
}

func (g *Game) Check(userID database.UserID) {
	g.sendPlayerEvent(playerEvent{UserID: userID, Check: true})
}

func (g *Game) AllIn(userID database.UserID) {
	g.sendPlayerEvent(playerEvent{UserID: userID, AllIn: true})
}

func (g *Game) Raise(userID database.UserID) {
	g.sendPlayerEvent(playerEvent{UserID: userID, Raise: true})
}

func (g *Game) Bet(userID database.UserID, bet database.PokerChip) {
	g.sendPlayerEvent(playerEvent{UserID: userID, Bet: bet})
}

func (g *Game) Call(userID database.UserID) {
	g.sendPlayerEvent(playerEvent{UserID: userID, Call: true})
}

func (g *Game) Fold(userID database.UserID) {
	g.sendPlayerEvent(playerEvent{UserID: userID, Fold: true})
}

func (g *Game) sendUnsitPlayerEvent(userID database.UserID) {
	g.sendPlayerEvent(playerEvent{UserID: userID, Unsit: true})
}

func (g *Game) sendPlayerEvent(evt playerEvent) {
	select {
	case g.playersEventCh <- evt:
	default:
	}
}

func (g *ongoingGame) isHeadsUpGame() bool {
	return len(g.players) == 2 // https://en.wikipedia.org/wiki/Heads-up_poker
}

func (g *ongoingGame) computeWinners() (winner []gameResult) {
	return computeWinners(g.players, g.communityCards)
}

func computeWinners(players []*PokerPlayer, communityCards []string) (winner []gameResult) {
	countAlive := 0
	var lastAlive *PokerPlayer
	for _, p := range players {
		if !p.folded.Load() {
			countAlive++
			lastAlive = p
		}
	}
	if countAlive == 0 {
		return []gameResult{}
	} else if countAlive == 1 {
		return []gameResult{{-1, []*PokerPlayer{lastAlive}}}
	}

	m := make(map[int32][]*PokerPlayer)
	for _, p := range players {
		if p.folded.Load() {
			continue
		}

		var playerCard1, playerCard2 string
		p.cards.RWith(func(pCards []playerCard) {
			playerCard1 = pCards[0].name
			playerCard2 = pCards[1].name
		})

		if len(communityCards) != 5 {
			return []gameResult{}
		}
		hand := []poker.Card{
			poker.NewCard(cardToPokerCard(communityCards[0])),
			poker.NewCard(cardToPokerCard(communityCards[1])),
			poker.NewCard(cardToPokerCard(communityCards[2])),
			poker.NewCard(cardToPokerCard(communityCards[3])),
			poker.NewCard(cardToPokerCard(communityCards[4])),
			poker.NewCard(cardToPokerCard(playerCard1)),
			poker.NewCard(cardToPokerCard(playerCard2)),
		}
		handEvaluation := poker.Evaluate(hand)
		if _, ok := m[handEvaluation]; !ok {
			m[handEvaluation] = make([]*PokerPlayer, 0)
		}
		m[handEvaluation] = append(m[handEvaluation], p)
	}

	arr := make([]gameResult, 0)
	for k, v := range m {
		arr = append(arr, gameResult{handScore: k, players: v})
	}
	sortGameResults(arr)

	return arr
}

// Sort players by cash remaining (to have all-ins first), then by GameBet.
func sortGameResults(arr []gameResult) {
	for idx := range arr {
		sort.Slice(arr[idx].players, func(i, j int) bool {
			if arr[idx].players[i].getCash() == arr[idx].players[j].getCash() {
				return arr[idx].players[i].gameBet < arr[idx].players[j].gameBet
			}
			return arr[idx].players[i].getCash() < arr[idx].players[j].getCash()
		})
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i].handScore < arr[j].handScore })
}

func (g *ongoingGame) getDeckStr() string {
	return strings.Join(g.deck, "")
}

func (g *ongoingGame) GetDeckHash() string {
	return utils.MD5([]byte(g.getDeckStr()))
}

// Get the player index in ongoingGame.Players from a seat index (index in Game.Players)
// [nil p1 nil nil p2 nil] -> Game.Players
// [p1 p2]                 -> ongoingGame.Players
func (g *ongoingGame) getPlayerBySeatIdx(seatIdx int) (*PokerPlayer, int) {
	for idx, p := range g.players {
		if p.seatIdx == seatIdx {
			return p, idx
		}
	}
	return nil, -1
}

func (g *ongoingGame) countCanBetPlayers() (nbCanBet int) {
	for _, p := range g.players {
		if p.canBet() {
			nbCanBet++
		}
	}
	return
}

func (g *ongoingGame) countAlivePlayers() (playerAlive int) {
	return countAlivePlayers(g.players)
}

func countAlivePlayers(players []*PokerPlayer) (playerAlive int) {
	for _, p := range players {
		if !p.folded.Load() {
			playerAlive++
		}
	}
	return
}

// IsSeatedUnsafe returns either or not a userID is seated at the table.
// WARN: The caller of this function needs to ensure that g.Players has been Lock/RLock
func (g *Game) IsSeatedUnsafe(userID database.UserID) bool {
	return isSeated(*g.Players.Val(), userID)
}

func (g *Game) IsSeated(userID database.UserID) bool {
	return isSeated(g.Players.Get(), userID)
}

func isSeated(players seatedPlayers, userID database.UserID) bool {
	return players.get(userID) != nil
}

func isRoundSettled(players []*PokerPlayer) bool {
	arr := make([]*PokerPlayer, len(players))
	copy(arr, players)
	sort.Slice(arr, func(i, j int) bool { return arr[i].GetBet() > arr[j].GetBet() })
	b := arr[0].GetBet()
	for _, el := range arr {
		if !el.canBet() {
			continue
		}
		if el.GetBet() != b {
			return false
		}
	}
	return true
}

func (g *Game) incrDealerIdx() (smallBlindIdx, bigBlindIdx int) {
	ongoing := g.ongoing
	nbPlayers := len(ongoing.players)
	dealerSeatIdx := g.dealerSeatIdx.Load()
	var dealerPlayer *PokerPlayer
	var dealerIdx int
	for {
		dealerSeatIdx = (dealerSeatIdx + 1) % NbPlayers
		if dealerPlayer, dealerIdx = ongoing.getPlayerBySeatIdx(int(dealerSeatIdx)); dealerPlayer != nil {
			break
		}
	}
	g.dealerSeatIdx.Store(dealerSeatIdx)
	startIDx := utils.Ternary(ongoing.isHeadsUpGame(), 0, 1)
	smallBlindIdx = (dealerIdx + startIDx) % nbPlayers
	bigBlindIdx = (dealerIdx + startIDx + 1) % nbPlayers
	return
}

func (g *Game) Sit(userID database.UserID, username database.Username, pokerReferredBy *database.UserID, pos int) {
	if err := g.Players.WithE(func(gPlayers *seatedPlayers) error {
		pokerTable, err := g.db.GetPokerTableBySlug(g.roomID.String())
		if err != nil {
			return errors.New("failed to get poker table")
		}
		tableAccount, err := g.db.GetPokerTableAccount(userID, pokerTable.ID)
		if err != nil {
			return errors.New("failed to get table account")
		}
		if tableAccount.Amount < pokerTable.MinBet {
			return errors.New(fmt.Sprintf("not enough chips to sit. have: %d, need: %d", tableAccount.Amount, pokerTable.MinBet))
		}
		if isSeated(*gPlayers, userID) {
			return errors.New("player already seated")
		}
		if pos < 0 || pos >= len(*gPlayers) {
			return errors.New("invalid position")
		}
		if (*gPlayers)[pos] != nil {
			return errors.New("seat already taken")
		}
		(*gPlayers)[pos] = &seatedPlayer{
			seatIdx:         pos,
			userID:          userID,
			username:        username,
			cash:            rwmtx.RWMtxUInt64[database.PokerChip]{rwmtx.New(tableAccount.Amount)},
			lastActionTS:    time.Now(),
			pokerReferredBy: pokerReferredBy,
		}

		PubSub.Pub(g.roomID.Topic(), PokerSeatTakenEvent{})
		g.newLogEvent(fmt.Sprintf("%s sit", username.String()))

		return nil
	}); err != nil {
		PubSub.Pub(g.roomID.UserTopic(userID), NewErrorMsgEvent(err.Error()))
		return
	}
}

func (g *Game) UnSit(userID database.UserID) {
	g.Players.With(func(gPlayers *seatedPlayers) {
		if p := gPlayers.get(userID); p != nil {
			g.unSitPlayer(gPlayers, p)
			g.newLogEvent(fmt.Sprintf("%s un-sit", p.username.String()))
		}
	})
}

func (g *Game) unSitPlayer(gPlayers *seatedPlayers, seatedPlayer *seatedPlayer) {
	ongoing := g.ongoing
	if ongoing != nil {
		if player := ongoing.players.get(seatedPlayer.userID); player != nil {
			g.sendUnsitPlayerEvent(player.userID)
			player.unsit.Store(true)
			player.folded.Store(true)
			player.cards.RWith(func(playerCards []playerCard) {
				for _, card := range playerCards {
					evt := PokerEvent{ID: card.idx, Name: "", ZIdx: card.zIdx, Top: BurnStackY, Left: BurnStackX, Angle: "0deg", Reveal: false}
					PubSub.Pub(g.roomID.Topic(), evt)
					ongoing.events.Append(evt)
				}
			})
		}
	}
	(*gPlayers)[seatedPlayer.seatIdx] = nil
	PubSub.Pub(g.roomID.Topic(), PokerSeatLeftEvent{})
}

func generateDeck() []string {
	deck := []string{
		"A♠", "2♠", "3♠", "4♠", "5♠", "6♠", "7♠", "8♠", "9♠", "10♠", "J♠", "Q♠", "K♠",
		"A♥", "2♥", "3♥", "4♥", "5♥", "6♥", "7♥", "8♥", "9♥", "10♥", "J♥", "Q♥", "K♥",
		"A♣", "2♣", "3♣", "4♣", "5♣", "6♣", "7♣", "8♣", "9♣", "10♣", "J♣", "Q♣", "K♣",
		"A♦", "2♦", "3♦", "4♦", "5♦", "6♦", "7♦", "8♦", "9♦", "10♦", "J♦", "Q♦", "K♦",
	}
	r := rand.New(utils.NewCryptoRandSource())
	utils.Shuffle1(r, deck)
	return deck
}

func newOngoing(eligiblePlayers seatedPlayers) *ongoingGame {
	return &ongoingGame{
		deck:          generateDeck(),
		players:       eligiblePlayers.toPokerPlayers(),
		waitTurnEvent: rwmtx.New(PokerWaitTurnEvent{Idx: -1}),
		createdAt:     time.Now(),
	}
}

func (g *Game) newLogEvent(msg string) {
	ongoing := g.ongoing
	logEvt := LogEvent{Message: msg}
	PubSub.Pub(g.roomID.LogsTopic(), logEvt)
	if ongoing != nil {
		ongoing.logEvents.Append(logEvt)
	}
}

func showCards(g *Game, seats []Seat) {
	ongoing := g.ongoing
	roomTopic := g.roomID.Topic()
	for _, p := range ongoing.players {
		if !p.folded.Load() {
			var firstCard, secondCard playerCard
			p.cards.RWith(func(pCards []playerCard) {
				firstCard = pCards[0]
				secondCard = pCards[1]
			})
			seatData := seats[p.seatIdx]
			if p.seatIdx == 0 {
				seatData.Left -= 30
			} else if p.seatIdx == 1 {
				seatData.Left -= 31
			} else if p.seatIdx == 2 {
				seatData.Top -= 8
			}
			evt1 := PokerEvent{ID: firstCard.idx, Name: firstCard.name, ZIdx: firstCard.zIdx, Top: seatData.Top, Left: seatData.Left, Reveal: true}
			evt2 := PokerEvent{ID: secondCard.idx, Name: secondCard.name, ZIdx: secondCard.zIdx, Top: seatData.Top, Left: seatData.Left + 53, Reveal: true}
			PubSub.Pub(roomTopic, evt1)
			PubSub.Pub(roomTopic, evt2)
			ongoing.events.Append(evt1, evt2)
		}
	}
}

func setWaitTurn(g *Game, seatIdx int) {
	evt := PokerWaitTurnEvent{Idx: seatIdx, CreatedAt: time.Now()}
	PubSub.Pub(g.roomID.Topic(), evt)
	g.ongoing.waitTurnEvent.Set(evt)
}

func setAutoAction(g *Game, roomUserTopic, msg string) {
	evt := AutoActionEvent{Message: msg}
	PubSub.Pub(roomUserTopic, evt)
	g.ongoing.autoActionEvent.Set(evt)
}

type PlayerAction int

const (
	NoAction PlayerAction = iota
	FoldAction
	CallAction
	CheckAction
	BetAction
	AllInAction
	RaiseAction
)

func (a PlayerAction) String() string {
	switch a {
	case NoAction:
		return ""
	case FoldAction:
		return "fold"
	case CallAction:
		return "call"
	case CheckAction:
		return "check"
	case BetAction:
		return "bet"
	case RaiseAction:
		return "raise"
	case AllInAction:
		return "all-in"
	}
	return ""
}

const (
	doNothing = iota
	breakRoundIsSettledLoop
	continueGetPlayerEventLoop
	breakGetPlayerEventLoop
)

type autoAction struct {
	action PlayerAction
	evt    playerEvent
}

func foldPlayer(g *Game, p *PokerPlayer) {
	roomTopic := g.roomID.Topic()
	p.folded.Store(true)
	var firstCard, secondCard playerCard
	p.cards.RWith(func(pCards []playerCard) {
		firstCard = pCards[0]
		secondCard = pCards[1]
	})
	evt1 := PokerEvent{ID: firstCard.idx, Name: "", ZIdx: firstCard.zIdx, Top: BurnStackY, Left: BurnStackX, Angle: "0deg", Reveal: false}
	evt2 := PokerEvent{ID: secondCard.idx, Name: "", ZIdx: secondCard.zIdx, Top: BurnStackY, Left: BurnStackX, Angle: "0deg", Reveal: false}
	PubSub.Pub(roomTopic, evt1)
	PubSub.Pub(roomTopic, evt2)
	g.ongoing.events.Append(evt1, evt2)
}

func doUnsit(g *Game, p *PokerPlayer, playerAlive *int) int {
	*playerAlive = g.ongoing.countAlivePlayers()
	if *playerAlive == 1 {
		p.countChancesToAction--
		return breakRoundIsSettledLoop
	}
	return continueGetPlayerEventLoop
}

func doTimeout(g *Game, p *PokerPlayer, playerAlive *int) int {
	pUsername := p.username
	if p.GetBet() < g.ongoing.minBet.Get() {
		foldPlayer(g, p)
		p.status.Set("fold")
		g.newLogEvent(fmt.Sprintf("%s auto fold", pUsername))

		*playerAlive--
		if *playerAlive == 1 {
			return breakRoundIsSettledLoop
		}
		return doNothing
	}
	p.hasChecked = true
	p.status.Set("check")
	g.newLogEvent(fmt.Sprintf("%s auto check", pUsername))
	return doNothing
}

func doCheck(g *Game, p *PokerPlayer) int {
	minBet := g.ongoing.minBet.Get()
	if p.GetBet() < minBet {
		msg := fmt.Sprintf("Need to bet %d", minBet-p.GetBet())
		PubSub.Pub(g.roomID.UserTopic(p.userID), NewErrorMsgEvent(msg))
		return continueGetPlayerEventLoop
	}
	p.hasChecked = true
	p.status.Set("check")
	g.newLogEvent(fmt.Sprintf("%s check", p.username))
	return doNothing
}

func doFold(g *Game, p *PokerPlayer, playerAlive *int) int {
	roomUserTopic := g.roomID.UserTopic(p.userID)
	if p.GetBet() == g.ongoing.minBet.Get() {
		msg := fmt.Sprintf("Cannot fold if there is no bet; check")
		PubSub.Pub(roomUserTopic, NewErrorMsgEvent(msg))
		return doCheck(g, p)
	}
	foldPlayer(g, p)
	p.status.Set("fold")
	g.newLogEvent(fmt.Sprintf("%s fold", p.username))

	*playerAlive--
	if *playerAlive == 1 {
		PubSub.Pub(roomUserTopic, NewErrorMsgEvent(""))
		return breakRoundIsSettledLoop
	}
	return doNothing
}

func doCall(g *Game, p *PokerPlayer,
	newlyAllInPlayers *[]*PokerPlayer, lastBetPlayerIdx *int, playerToPlayIdx int) int {
	pUsername := p.username
	bet := utils.MinInt(g.ongoing.minBet.Get()-p.GetBet(), p.getCash())
	if bet == 0 {
		return doCheck(g, p)
	} else if bet == p.cash.Get() {
		return doAllIn(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx)
	} else {
		p.status.Set("call")
		p.doBetAndNotif(g, bet)
		logMsg := fmt.Sprintf("%s call (%d)", pUsername, bet)
		g.newLogEvent(logMsg)
	}
	return doNothing
}

func doAllIn(g *Game, p *PokerPlayer,
	newlyAllInPlayers *[]*PokerPlayer, lastBetPlayerIdx *int, playerToPlayIdx int) int {
	bet := p.getCash()
	minBet := g.ongoing.minBet.Get()
	if (p.GetBet() + bet) > minBet {
		*lastBetPlayerIdx = playerToPlayIdx
		g.ongoing.minRaise.Set(bet)
		PubSub.Pub(g.roomID.Topic(), PokerMinRaiseUpdatedEvent{MinRaise: bet})
	}
	g.ongoing.minBet.Set(utils.MaxInt(p.GetBet()+bet, minBet))
	p.doBetAndNotif(g, bet)
	logMsg := fmt.Sprintf("%s all-in (%d)", p.username, bet)
	if p.isAllIn() {
		*newlyAllInPlayers = append(*newlyAllInPlayers, p)
	}
	p.status.Set("all-in")
	g.newLogEvent(logMsg)
	return doNothing
}

func doRaise(g *Game, p *PokerPlayer,
	newlyAllInPlayers *[]*PokerPlayer, lastBetPlayerIdx *int, playerToPlayIdx int) int {
	return doBet(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx, g.ongoing.minRaise.Get())
}

func doBet(g *Game, p *PokerPlayer,
	newlyAllInPlayers *[]*PokerPlayer, lastBetPlayerIdx *int, playerToPlayIdx int, evtBet database.PokerChip) int {
	roomUserTopic := g.roomID.UserTopic(p.userID)
	minBet := g.ongoing.minBet.Get()
	minRaise := g.ongoing.minRaise.Get()
	playerBet := p.bet.Get()          // Player's chips already on the table
	callDelta := minBet - playerBet   // Chips missing to equalize the minBet
	bet := evtBet + callDelta         // Amount of chips player need to put on the table to make the raise
	playerTotalBet := bet + playerBet // Player's total bet during the betting round
	if bet >= p.cash.Get() {
		return doAllIn(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx)
	}
	betLbl := utils.Ternary(g.IsBet(), "bet", "raise")
	// Ensure the player cannot bet below the table minimum bet (amount of the big blind)
	if evtBet < minRaise {
		msg := fmt.Sprintf("%s (%d) is too low. Must %s at least %d", betLbl, evtBet, betLbl, minRaise)
		PubSub.Pub(roomUserTopic, NewErrorMsgEvent(msg))
		return continueGetPlayerEventLoop
	}
	*lastBetPlayerIdx = playerToPlayIdx
	PubSub.Pub(g.roomID.Topic(), PokerMinRaiseUpdatedEvent{MinRaise: evtBet})
	g.ongoing.minRaise.Set(evtBet)
	g.ongoing.minBet.Set(playerTotalBet)

	p.doBetAndNotif(g, bet)
	g.newLogEvent(fmt.Sprintf("%s %s %d", p.username, betLbl, g.ongoing.minRaise.Get()))
	if p.hasChecked {
		p.status.Set("check-" + betLbl)
		p.hasChecked = false
	} else {
		p.status.Set(betLbl)
	}
	g.ongoing.hasBet.Set(true)
	return doNothing
}

func handleAutoActionReceived(g *Game, autoCache map[database.UserID]autoAction, evt playerEvent) int {
	roomUserTopic := g.roomID.UserTopic(evt.UserID)
	autoActionVal := autoCache[evt.UserID]
	if evt.Fold && autoActionVal.action == FoldAction ||
		evt.Call && autoActionVal.action == CallAction ||
		evt.Check && autoActionVal.action == CheckAction {
		delete(autoCache, evt.UserID)
		setAutoAction(g, roomUserTopic, "")
		return continueGetPlayerEventLoop
	}

	action := evt.getAction()
	if action != NoAction {
		autoCache[evt.UserID] = autoAction{action: action, evt: evt}
		msg := "Will auto "
		if action == FoldAction {
			msg += "fold/check"
		} else {
			msg += action.String()
		}
		if evt.Bet > 0 {
			msg += fmt.Sprintf(" %d", evt.Bet.Raw())
		}
		setAutoAction(g, roomUserTopic, msg)
	}
	return continueGetPlayerEventLoop
}

func applyAutoAction(g *Game, p *PokerPlayer,
	newlyAllInPlayers *[]*PokerPlayer,
	lastBetPlayerIdx, playerAlive *int, playerToPlayIdx int, autoAction autoAction,
	autoCache map[database.UserID]autoAction) (actionResult int) {

	pUserID := p.userID
	roomUserTopic := g.roomID.UserTopic(pUserID)
	if autoAction.action > NoAction {
		time.Sleep(500 * time.Millisecond)
		actionResult = handlePlayerActionEvent(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerAlive, playerToPlayIdx, autoAction.evt)
	}
	delete(autoCache, pUserID)
	setAutoAction(g, roomUserTopic, "")
	return actionResult
}

func handlePlayerActionEvent(g *Game, p *PokerPlayer,
	newlyAllInPlayers *[]*PokerPlayer,
	lastBetPlayerIdx, playerAlive *int, playerToPlayIdx int, evt playerEvent) (actionResult int) {

	p.lastActionTS = time.Now()
	if evt.Fold {
		actionResult = doFold(g, p, playerAlive)
	} else if evt.Check {
		actionResult = doCheck(g, p)
	} else if evt.Call {
		actionResult = doCall(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx)
	} else if evt.AllIn {
		actionResult = doAllIn(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx)
	} else if evt.Raise {
		actionResult = doRaise(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx)
	} else if evt.Bet > 0 {
		actionResult = doBet(g, p, newlyAllInPlayers, lastBetPlayerIdx, playerToPlayIdx, evt.Bet)
	} else {
		actionResult = continueGetPlayerEventLoop
	}
	return actionResult
}

// Return either or not the game ended because only 1 player left playing (or none)
func execBettingRound(g *Game, skip int, minBet database.PokerChip) bool {
	roomID := g.roomID
	roomTopic := roomID.Topic()
	gPokerTableMinBet := g.pokerTableMinBet
	g.ongoing.minBet.Set(minBet)
	g.ongoing.minRaise.Set(gPokerTableMinBet)
	PubSub.Pub(roomTopic, PokerMinRaiseUpdatedEvent{MinRaise: gPokerTableMinBet})
	db := g.db
	ongoing := g.ongoing
	_, dealerIdx := ongoing.getPlayerBySeatIdx(int(g.dealerSeatIdx.Load()))
	playerToPlayIdx := (dealerIdx + skip) % len(ongoing.players)
	lastBetPlayerIdx := -1
	newlyAllInPlayers := make([]*PokerPlayer, 0)
	autoCache := make(map[database.UserID]autoAction)

	for _, p := range ongoing.players {
		p.hasChecked = false
		if p.canBet() {
			p.status.Set("")
		}
	}
	PubSub.Pub(roomTopic, RedrawSeatsEvent{})

	playerAlive := ongoing.countAlivePlayers()

	// Avoid asking for actions if only 1 player can do so (because others are all-in)
	nbCanBet := ongoing.countCanBetPlayers()
	if nbCanBet == 0 || nbCanBet == 1 {
		goto RoundIsSettled
	}

	// TODO: implement maximum re-raise

RoundIsSettledLoop:
	for { // Repeat until the round is settled (all players have equals bet or fold or all-in)
	AllPlayersLoop:
		for { // Repeat until all players have played
			playerToPlayIdx = (playerToPlayIdx + 1) % len(ongoing.players)
			p := ongoing.players[playerToPlayIdx]
			g.ongoing.playerToPlay.Set(p.userID)
			p.countChancesToAction++
			pUserID := p.userID
			roomUserTopic := roomID.UserTopic(pUserID)

			if playerToPlayIdx == lastBetPlayerIdx {
				break AllPlayersLoop
			}
			lastBetPlayerIdx = utils.Ternary(lastBetPlayerIdx == -1, playerToPlayIdx, lastBetPlayerIdx)
			if !p.canBet() {
				continue AllPlayersLoop
			}

			minBet = g.ongoing.minBet.Get()

			PubSub.Pub(roomUserTopic, RefreshButtonsEvent{})

			setWaitTurn(g, p.seatIdx)
			PubSub.Pub(roomUserTopic, PokerYourTurnEvent{})

			// Maximum time allowed for the player to send his action
			waitCh := time.After(MaxUserCountdown * time.Second)
		GetPlayerEventLoop:
			for { // Repeat until we get an event from the player we're interested in
				var evt playerEvent
				actionResult := doNothing
				// Check for pre-selected action
				if autoActionVal, ok := autoCache[pUserID]; ok {
					actionResult = applyAutoAction(g, p, &newlyAllInPlayers,
						&lastBetPlayerIdx, &playerAlive, playerToPlayIdx, autoActionVal, autoCache)
					goto checkActionResult
				}
				select {
				case evt = <-g.playersEventCh:
				case <-waitCh: // Waited too long, either auto-check or auto-fold
					actionResult = doTimeout(g, p, &playerAlive)
					goto checkActionResult
				}
				if evt.Unsit {
					actionResult = doUnsit(g, p, &playerAlive)
					goto checkActionResult
				}
				if evt.UserID != pUserID {
					actionResult = handleAutoActionReceived(g, autoCache, evt)
					goto checkActionResult
				}
				actionResult = handlePlayerActionEvent(g, p, &newlyAllInPlayers,
					&lastBetPlayerIdx, &playerAlive, playerToPlayIdx, evt)
				goto checkActionResult

			checkActionResult:
				switch actionResult {
				case doNothing:
				case continueGetPlayerEventLoop:
					continue GetPlayerEventLoop
				case breakGetPlayerEventLoop:
					break GetPlayerEventLoop
				case breakRoundIsSettledLoop:
					break RoundIsSettledLoop
				}
				PubSub.Pub(roomUserTopic, NewErrorMsgEvent(""))
				PubSub.Pub(roomTopic, RedrawSeatsEvent{})
				break GetPlayerEventLoop
			} // End of repeat until we get an event from the player we're interested in
		} // End of repeat until all players have played
		// All settle when all players have the same bet amount
		if isRoundSettled(ongoing.players) {
			break RoundIsSettledLoop
		}
	} // End of repeat until the round is settled (all players have equals bet or fold or all-in)

RoundIsSettled:

	setAutoAction(g, roomTopic, "")
	PubSub.Pub(roomTopic, NewErrorMsgEvent(""))
	g.newLogEvent(fmt.Sprintf("--"))
	setWaitTurn(g, -1)

	time.Sleep(animationTime)

	mainPot := ongoing.mainPot.Get()

	// Calculate what is the max gain all-in players can make
	computeAllInMaxGain(ongoing, newlyAllInPlayers, mainPot)

	// Always refund the difference between the first-biggest bet and the second-biggest bet.
	// We refund the "uncalled bet" so that it does not go in the main pot and does not get raked.
	// Also, if a player goes all-in and a fraction of his bet is not matched, it will be refunded.
	refundUncalledBet(db, ongoing, g.pokerTableID, roomTopic)

	// Transfer players bets into the main pot
	mainPot += resetPlayersBet(ongoing)

	PubSub.Pub(roomTopic, PokerMainPotUpdatedEvent{MainPot: mainPot})
	ongoing.mainPot.Set(mainPot)
	g.ongoing.hasBet.Set(false)

	return playerAlive <= 1
}

// Reset all players bets, and return the sum of it
func resetPlayersBet(ongoing *ongoingGame) (sum database.PokerChip) {
	for _, p := range ongoing.players {
		sum += p.resetBet()
	}
	return
}

func refundUncalledBet(db *database.DkfDB, ongoing *ongoingGame, pokerTableID int64, roomTopic string) {
	lenPlayers := len(ongoing.players)
	if lenPlayers < 2 {
		return
	}
	newArray := make([]*PokerPlayer, lenPlayers)
	copy(newArray, ongoing.players)
	sort.Slice(newArray, func(i, j int) bool { return newArray[i].GetBet() > newArray[j].GetBet() })
	firstPlayer := newArray[0]
	secondPlayer := newArray[1]
	diff := firstPlayer.GetBet() - secondPlayer.GetBet()
	if diff > 0 {
		firstPlayer.refundPartialBet(db, pokerTableID, diff)
		PubSub.Pub(roomTopic, RedrawSeatsEvent{})
		time.Sleep(animationTime)
	}
}

type Seat struct {
	Top   int
	Left  int
	Angle string
	Top2  int
	Left2 int
}

// Positions of the dealer token for each seats
var dealerTokenPos = [][]int{
	{142, 714},
	{261, 732},
	{384, 607},
	{369, 379},
	{367, 190},
	{363, 123},
}

func burnCard(g *Game, idx, burnIdx *int) {
	ongoing := g.ongoing
	*idx++
	evt := PokerEvent{
		ID:   *idx,
		ID1:  *idx + 1,
		Name: "",
		ZIdx: *idx + 53,
		Top:  BurnStackY + (*burnIdx * 2),
		Left: BurnStackX + (*burnIdx * 4),
	}
	PubSub.Pub(g.roomID.Topic(), evt)
	ongoing.events.Append(evt)
	*burnIdx++
}

func dealCard(g *Game, idx *int, dealCardIdx int) {
	ongoing := g.ongoing
	card := ongoing.deck[*idx]
	*idx++
	evt := PokerEvent{
		ID:     *idx,
		ID1:    *idx + 1,
		Name:   card,
		ZIdx:   *idx + 53,
		Top:    DealY,
		Left:   DealX + (dealCardIdx * DealSpacing),
		Reveal: true,
	}
	PubSub.Pub(g.roomID.Topic(), evt)
	ongoing.events.Append(evt)
	ongoing.communityCards = append(ongoing.communityCards, card)
}

func dealPlayersCards(g *Game, seats []Seat, idx *int) {
	roomID := g.roomID
	ongoing := g.ongoing
	roomTopic := roomID.Topic()
	var card string
	for cardIdx := 1; cardIdx <= NbCardsPerPlayer; cardIdx++ {
		for _, p := range ongoing.players {
			pUserID := p.userID
			if !p.canBet() {
				continue
			}
			if p.unsit.Load() {
				continue
			}
			roomUserTopic := roomID.UserTopic(pUserID)
			seatData := seats[p.seatIdx]
			time.Sleep(animationTime)
			card = ongoing.deck[*idx]
			*idx++
			left := seatData.Left
			top := seatData.Top
			if cardIdx == 2 {
				left = seatData.Left2
				top = seatData.Top2
			}

			seatData1 := seats[p.seatIdx]
			if p.seatIdx == 0 {
				seatData1.Left -= 30
			} else if p.seatIdx == 1 {
				seatData1.Left -= 31
			} else if p.seatIdx == 2 {
				seatData1.Top -= 8
			}
			if cardIdx == 2 {
				seatData1.Left += 53
			}

			evt := PokerEvent{ID: *idx, ID1: *idx + 1, Name: "", ZIdx: *idx + 104, Top: top, Left: left, Angle: seatData.Angle}
			evt1 := PokerEvent{ID: *idx, ID1: *idx + 1, Name: card, ZIdx: *idx + 104, Top: seatData1.Top, Left: seatData1.Left, Reveal: true, UserID: pUserID}

			PubSub.Pub(roomTopic, evt)
			PubSub.Pub(roomUserTopic, evt1)

			p.cards.Append(playerCard{idx: *idx, zIdx: *idx + 104, name: card})

			ongoing.events.Append(evt, evt1)
		}
	}
}

func computeAllInMaxGain(ongoing *ongoingGame, newlyAllInPlayers []*PokerPlayer, mainPot database.PokerChip) {
	for _, p := range newlyAllInPlayers {
		maxGain := mainPot
		for _, op := range ongoing.players {
			maxGain += utils.MinInt(op.GetBet(), p.GetBet())
		}
		p.allInMaxGain = maxGain
	}
}

func dealerThread(g *Game, eligiblePlayers seatedPlayers) {
	eligiblePlayers.resetStatuses()
	g.ongoing = newOngoing(eligiblePlayers)

	roomID := g.roomID
	roomTopic := roomID.Topic()
	bigBlindBet := g.pokerTableMinBet
	collectRake := false
	ongoing := g.ongoing
	isHeadsUpGame := ongoing.isHeadsUpGame()

	seats := []Seat{
		{Top: 55, Left: 610, Top2: 55 + 5, Left2: 610 + 5, Angle: "-95deg"},
		{Top: 175, Left: 620, Top2: 175 + 5, Left2: 620 + 3, Angle: "-80deg"},
		{Top: 290, Left: 580, Top2: 290 + 5, Left2: 580 + 1, Angle: "-50deg"},
		{Top: 310, Left: 430, Top2: 310 + 5, Left2: 430 + 1, Angle: "0deg"},
		{Top: 315, Left: 240, Top2: 315 + 5, Left2: 240 + 1, Angle: "0deg"},
		{Top: 270, Left: 70, Top2: 270 + 5, Left2: 70 + 1, Angle: "10deg"},
	}

	idx := 0
	burnIdx := 0

	sbIdx, bbIdx := g.incrDealerIdx()

	PubSub.Pub(roomTopic, GameStartedEvent{DealerSeatIdx: int(g.dealerSeatIdx.Load())})
	g.newLogEvent(fmt.Sprintf("-- New game --"))

	applySmallBlindBet(g, bigBlindBet, sbIdx)
	time.Sleep(animationTime)

	applyBigBlindBet(g, bigBlindBet, bbIdx)
	time.Sleep(animationTime)
	g.ongoing.hasBet.Set(true)
	g.ongoing.minRaise.Set(bigBlindBet)

	// Deal players cards
	dealPlayersCards(g, seats, &idx)

	PubSub.Pub(roomTopic, RefreshButtonsEvent{})

	// Wait for players to bet/call/check/fold...
	time.Sleep(animationTime)
	skip := utils.Ternary(isHeadsUpGame, 1, 2)
	if execBettingRound(g, skip, bigBlindBet) {
		goto END
	}

	// Flop (3 first cards)
	time.Sleep(animationTime)
	burnCard(g, &idx, &burnIdx)
	for i := 1; i <= 3; i++ {
		time.Sleep(animationTime)
		dealCard(g, &idx, i)
	}

	// No flop, no drop
	if g.tableType == TableTypeRake {
		collectRake = true
	}

	skip = utils.Ternary(isHeadsUpGame, 1, 0)

	// Wait for players to bet/call/check/fold...
	time.Sleep(animationTime)
	if execBettingRound(g, skip, 0) {
		goto END
	}

	// Turn (4th card)
	time.Sleep(animationTime)
	burnCard(g, &idx, &burnIdx)
	time.Sleep(animationTime)
	dealCard(g, &idx, 4)

	// Wait for players to bet/call/check/fold...
	time.Sleep(animationTime)
	if execBettingRound(g, skip, 0) {
		goto END
	}

	// River (5th card)
	time.Sleep(animationTime)
	burnCard(g, &idx, &burnIdx)
	time.Sleep(animationTime)
	dealCard(g, &idx, 5)

	// Wait for players to bet/call/check/fold...
	time.Sleep(animationTime)
	if execBettingRound(g, skip, 0) {
		goto END
	}

	// Show cards
	showCards(g, seats)
	g.newLogEvent(g.ongoing.gameStr())

END:

	winners := ongoing.computeWinners()
	mainPotOrig := ongoing.mainPot.Get()
	mainPot, rake := computeRake(g.ongoing.players, bigBlindBet, mainPotOrig, collectRake)
	playersGain := processPot(winners, mainPot)
	winnersStr, winnerHand := applyGains(g, playersGain, mainPotOrig, rake)

	ongoing.mainPot.Set(0)

	PubSub.Pub(roomTopic, GameIsDoneEvent{Winner: winnersStr, WinnerHand: winnerHand})
	g.newLogEvent(fmt.Sprintf("-- Game ended --"))

	// Wait a minimum of X seconds before allowing a new game
	time.Sleep(MinTimeAfterGame * time.Second)

	// Auto unsit inactive players
	autoUnsitInactivePlayers(g)

	PubSub.Pub(roomTopic, GameIsOverEvent{})
	g.isGameStarted.Store(false)
}

func (g *ongoingGame) gameStr() string {
	out := fmt.Sprintf("%s", g.communityCards)
	for _, p := range g.players {
		if !p.folded.Load() {
			out += fmt.Sprintf(" | @%s", p.username)
			p.cards.RWith(func(pCards []playerCard) {
				out += " " + pCards[0].name
				out += " " + pCards[1].name
			})
		}
	}
	return out
}

func computeRake(players []*PokerPlayer, pokerTableMinBet, mainPotIn database.PokerChip, collectRake bool) (mainPotOut, rake database.PokerChip) {
	if !collectRake {
		return mainPotIn, 0
	}
	rake = calculateRake(mainPotIn, pokerTableMinBet, len(players))
	for _, p := range players {
		pctOfPot := float64(p.gameBet) / float64(mainPotIn)
		p.rakePaid = pctOfPot * float64(rake)
	}
	mainPotOut = mainPotIn - rake
	return mainPotOut, rake
}

func applySmallBlindBet(g *Game, bigBlindBet database.PokerChip, sbIdx int) {
	applyBlindBet(g, sbIdx, bigBlindBet/2, "small blind")
}

func applyBigBlindBet(g *Game, bigBlindBet database.PokerChip, bbIdx int) {
	applyBlindBet(g, bbIdx, bigBlindBet, "big blind")
}

func applyBlindBet(g *Game, playerIdx int, bet database.PokerChip, name string) {
	p := g.ongoing.players[playerIdx]
	p.doBetAndNotif(g, bet)
	g.newLogEvent(fmt.Sprintf("%s %s %d", p.username, name, bet))
}

func autoUnsitInactivePlayers(g *Game) {
	ongoing := g.ongoing
	pokerTableMinBet := g.pokerTableMinBet
	g.Players.With(func(gPlayers *seatedPlayers) {
		for _, p := range *gPlayers {
			if playerShouldBeBooted(p, ongoing, pokerTableMinBet) {
				g.unSitPlayer(gPlayers, p)
				g.newLogEvent(fmt.Sprintf("%s auto un-sit", p.username))
			}
		}
	})
}

// Returns either or not a seated player should be booted out of the table.
func playerShouldBeBooted(p *seatedPlayer, ongoing *ongoingGame, pokerTableMinBet database.PokerChip) (playerShallBeBooted bool) {
	if p == nil {
		return false
	}
	pIsEligible := p.isEligible(pokerTableMinBet)
	if !pIsEligible {
		return true
	}
	if p.lastActionTS.Before(ongoing.createdAt) {
		// If the player was playing the game, must be booted if he had the chance to make actions and did not.
		// If the player was not playing the game, must be booted if he's not eligible to play the next one.
		op := ongoing.players.get(p.userID)
		playerShallBeBooted = (op != nil && op.countChancesToAction > 0) ||
			(op == nil && !pIsEligible)
	}
	return playerShallBeBooted
}

const (
	TableTypeRake = iota
	TableType2
)

// Increase users rake-back and casino rake for paying tables.
func applyRake(g *Game, tx *database.DkfDB, rake database.PokerChip) {
	rakeBackMap := make(map[database.UserID]database.PokerChip)
	for _, p := range g.ongoing.players {
		if p.pokerReferredBy != nil {
			rakeBack := database.PokerChip(math.RoundToEven(RakeBackPct * p.rakePaid))
			rakeBackMap[*p.pokerReferredBy] += rakeBack
		}
	}
	casinoRakeBack := database.PokerChip(0)
	for userID, totalRakeBack := range rakeBackMap {
		casinoRakeBack += totalRakeBack
		rake -= totalRakeBack
		if err := tx.IncrUserRakeBack(userID, totalRakeBack); err != nil {
			logrus.Error(err)
			casinoRakeBack -= totalRakeBack
			rake += totalRakeBack
		}
	}
	_ = tx.IncrPokerCasinoRake(rake, casinoRakeBack)
}

func applyGains(g *Game, playersGain []PlayerGain, mainPot, rake database.PokerChip) (winnersStr, winnerHand string) {
	ongoing := g.ongoing
	pokerTableID := g.pokerTableID
	nbPlayersGain := len(playersGain)
	g.db.With(func(tx *database.DkfDB) {
		if nbPlayersGain >= 1 {
			winnerHand = utils.Ternary(nbPlayersGain == 1, playersGain[0].HandStr, "Split pot")

			if g.tableType == TableTypeRake {
				if !g.pokerTableIsTest {
					applyRake(g, tx, rake)
				}
				g.newLogEvent(fmt.Sprintf("Rake %d (%.2f%%)", rake, (float64(rake)/float64(mainPot))*100))
			}

			for _, el := range playersGain {
				g.newLogEvent(fmt.Sprintf("Winner #%d: %s %s -> %d", el.Group, el.Player.username, el.HandStr, el.Gain))
				winnersStr += el.Player.username.String() + " "
				el.Player.gain(tx, pokerTableID, el.Gain)
				pubCashBonus(g, el.Player.seatIdx, el.Gain, true)
			}
			for _, op := range ongoing.players {
				op.gain(tx, pokerTableID, 0)
			}

		} else if nbPlayersGain == 0 {
			// No winners, refund bets
			for _, op := range ongoing.players {
				op.refundBet(tx, pokerTableID)
			}
		}
	})
	return
}

type PlayerGain struct {
	Player  *PokerPlayer
	Gain    database.PokerChip
	Group   int
	HandStr string
}

func calculateRake(mainPot, pokerTableMinBet database.PokerChip, nbPlayers int) (rake database.PokerChip) {
	// https://www.pokerstars.com/poker/room/rake
	// BB: pct, 2P, 3-4P, 5+P
	rakeTable := map[database.PokerChip][]float64{
		3:    {0.035, 178, 178, 178},
		20:   {0.0415, 297, 297, 595},
		200:  {0.05, 446, 446, 1190},
		1000: {0.05, 722, 722, 1589},
		2000: {0.05, 892, 892, 1785},
	}
	maxRake := pokerTableMinBet * 15
	rakePct := 0.045
	if val, ok := rakeTable[pokerTableMinBet]; ok {
		rakePct = val[0]
		if nbPlayers == 2 {
			maxRake = database.PokerChip(val[1])
		} else if nbPlayers == 3 || nbPlayers == 4 {
			maxRake = database.PokerChip(val[2])
		} else if nbPlayers >= 5 {
			maxRake = database.PokerChip(val[3])
		}
	}
	rake = database.PokerChip(math.RoundToEven(rakePct * float64(mainPot)))
	rake = utils.MinInt(rake, maxRake) // Max rake
	return rake
}

func processPot(winners []gameResult, mainPot database.PokerChip) (res []PlayerGain) {
	if len(winners) == 0 {
		logrus.Error("winners has len 0")
		return
	}

	isOnlyPlayerAlive := len(winners) == 1 && len(winners[0].players) == 1
	for groupIdx, group := range winners {
		if mainPot == 0 {
			break
		}
		groupPlayers := group.players
		groupPlayersLen := len(groupPlayers)
		handStr := "Only player alive"
		if !isOnlyPlayerAlive {
			handStr = poker.RankString(group.handScore)
		}
		allInCount := 0
		calcExpectedSplit := func() database.PokerChip {
			return mainPot / utils.MaxInt(database.PokerChip(groupPlayersLen-allInCount), 1)
		}
		expectedSplit := calcExpectedSplit()
		for _, p := range groupPlayers {
			piece := utils.MinInt(p.maxGain(mainPot), expectedSplit)
			res = append(res, PlayerGain{Player: p, Gain: piece, Group: groupIdx, HandStr: handStr})
			mainPot -= piece
			if p.isAllIn() {
				allInCount++
				expectedSplit = calcExpectedSplit()
			}
		}
		// If everyone in the group was all-in, we need to evaluate the next group as well
		if allInCount == groupPlayersLen {
			continue
		}
		break
	}

	// If any remaining "odd chip(s)" distribute them to players.
	// TODO: these chips should be given to the stronger hand first
	idx := 0
	for mainPot > 0 {
		res[idx].Gain++
		mainPot--
		idx = (idx + 1) % len(res)
	}

	return
}

func cardToPokerCard(name string) string {
	r := strings.NewReplacer("♠", "s", "♥", "h", "♣", "c", "♦", "d", "10", "T")
	return r.Replace(name)
}

func (g *Game) OngoingPlayer(userID database.UserID) *PokerPlayer {
	if g.ongoing != nil {
		return g.ongoing.players.get(userID)
	}
	return nil
}

func (g *Game) Deal(userID database.UserID) {
	roomTopic := g.roomID.Topic()
	roomUserTopic := g.roomID.UserTopic(userID)
	eligiblePlayers := g.getEligibles()
	if !g.IsSeated(userID) {
		PubSub.Pub(roomUserTopic, NewErrorMsgEvent("you need to be seated"))
		return
	}
	if len(eligiblePlayers) < 2 {
		PubSub.Pub(roomUserTopic, NewErrorMsgEvent("need at least 2 players"))
		return
	}
	if !g.isGameStarted.CompareAndSwap(false, true) {
		PubSub.Pub(roomUserTopic, NewErrorMsgEvent("game already ongoing"))
		return
	}

	PubSub.Pub(roomUserTopic, NewErrorMsgEvent(""))
	PubSub.Pub(roomTopic, ResetCardsEvent{})
	time.Sleep(animationTime)

	go dealerThread(g, eligiblePlayers)
}

func (g *Game) CountSeated() (count int) {
	g.Players.RWith(func(gPlayers seatedPlayers) {
		for _, p := range gPlayers {
			if p != nil {
				count++
			}
		}
	})
	return
}

var PubSub = pubsub.NewPubSub[any]()

func Refund(db *database.DkfDB) {
	accounts, _ := db.GetPositivePokerTableAccounts()
	db.With(func(tx *database.DkfDB) {
		for _, account := range accounts {
			_ = tx.PokerTableAccountRefundPartialBet(account.UserID, account.PokerTableID, account.AmountBet)
		}
	})
}

type RoomID string

func (r RoomID) String() string    { return string(r) }
func (r RoomID) Topic() string     { return "room_" + string(r) }
func (r RoomID) LogsTopic() string { return r.Topic() + "_logs" }
func (r RoomID) UserTopic(userID database.UserID) string {
	return r.Topic() + "_" + userID.String()
}

func isHeartOrDiamond(name string) bool {
	return strings.Contains(name, "♥") ||
		strings.Contains(name, "♦")
}

func colorForCard(name string) string {
	return utils.Ternary(isHeartOrDiamond(name), "red", "black")
}

func buildDealerTokenHtml(g *Game) (html string) {
	html += `<div id="dealerToken"><div class="inner"></div></div>`
	if g.ongoing != nil {
		pos := dealerTokenPos[g.dealerSeatIdx.Load()]
		top := itoa(pos[0])
		left := itoa(pos[1])
		html += fmt.Sprintf(`<style>#dealerToken { top: %spx; left: %spx; }</style>`, top, left)
	}
	return
}

func BuildPayloadHtml(g *Game, authUser *database.User, payload any) (html string) {
	switch evt := payload.(type) {
	case GameStartedEvent:
		html += drawGameStartedEvent(evt, authUser)
	case GameIsDoneEvent:
		html += drawGameIsDoneHtml(g, evt)
	case GameIsOverEvent:
		html += drawGameIsOverHtml(g)
	case PlayerBetEvent:
		html += drawPlayerBetEvent(evt)
		html += drawSeatsStyle(authUser, g)
	case ErrorMsgEvent:
		html += drawErrorMsgEvent(evt)
	case AutoActionEvent:
		html += drawAutoActionMsgEvent(evt)
	case PlayerFoldEvent:
		html += drawPlayerFoldEvent(evt)
	case ResetCardsEvent:
		html += drawResetCardsEvent()
	case CashBonusEvent:
		html += drawCashBonus(evt)
	case RedrawSeatsEvent:
		html += drawSeatsStyle(authUser, g)
	case PokerSeatTakenEvent:
		html += drawSeatsStyle(authUser, g)
	case PokerSeatLeftEvent:
		html += drawSeatsStyle(authUser, g)
	case PokerWaitTurnEvent:
		html += drawCountDownStyle(evt)
	case PokerYourTurnEvent:
		html += drawYourTurnHtml(authUser)
	case PokerEvent:
		html += getPokerEventHtml(evt, animationTime.String())
	case PokerMainPotUpdatedEvent:
		html += drawMainPotHtml(evt)
	case PokerMinRaiseUpdatedEvent:
		html += drawMinRaiseHtml(evt)
	}
	return
}

func buildGameDiv(g *Game, authUser *database.User) (html string) {
	roomID := g.roomID
	html += `<div id="game">`
	html += `<div id="table"><div class="inner"></div><div class="cards-outline"></div></div>`
	html += buildSeatsHtml(g, authUser)
	html += buildCardsHtml()
	html += buildActionsDiv(roomID)
	html += buildDealerTokenHtml(g)
	html += buildMainPotHtml(g)
	html += buildMinRaiseHtml(g)
	html += buildWinnerHtml()
	html += `</div>`
	return
}

func BuildBaseHtml(g *Game, authUser *database.User, chatRoomSlug string) (html string) {
	ongoing := g.ongoing
	roomID := g.roomID
	html += hutils.HtmlCssReset
	html += pokerCss
	//html += `<script>document.onclick = function(e) { console.log(e.x, e.y); };</script>` // TODO: dev only
	//html += buildDevHtml()
	html += buildGameDiv(g, authUser)
	html += buildSoundsHtml(authUser)
	html += buildHelpHtml()
	html += `<div id="chat-div">`
	html += `	<iframe id="chat-top-bar" name="iframe1" src="/api/v1/chat/top-bar/` + chatRoomSlug + `" sandbox="allow-forms allow-scripts allow-same-origin allow-top-navigation-by-user-activation"></iframe>`
	html += `	<iframe id="chat-content" name="iframe2" src="/api/v1/chat/messages/` + chatRoomSlug + `/stream?hrm=1&hactions=1&hide_ts=1"></iframe>`
	html += `</div>`
	html += `<iframe src="/poker/` + roomID.String() + `/logs" id="eventLogs"></iframe>`

	if ongoing != nil {
		html += drawCountDownStyle(ongoing.waitTurnEvent.Get())
		html += drawAutoActionMsgEvent(ongoing.autoActionEvent.Get())
		ongoing.events.Each(func(evt PokerEvent) {
			if evt.UserID == 0 || evt.UserID == authUser.ID {
				html += getPokerEventHtml(evt, "0s")
			}
		})
	}
	return
}

func buildSoundsHtml(authUser *database.User) (html string) {
	html += `
<div id="soundsStatus">
	<a href="/settings/chat" rel="noopener noreferrer" target="_blank">`
	if authUser.PokerSoundsEnabled {
		html += `<img src="/public/img/sounds-enabled.png" style="height: 20px;" alt="" title="Sounds enabled" />`
	} else {
		html += `<img src="/public/img/no-sound.png" style="height: 20px;" alt="" title="Sounds disabled" />`
	}
	html += `</a>
</div>`
	return
}

func buildCardsHtml() (html string) {
	for i := 52; i >= 1; i-- {
		idxStr := itoa(i)
		html += fmt.Sprintf(`<div class="card-holder" id="card%s"><div class="back"><div class="inner"></div></div><div class="card"><div class="inner"></div></div></div>`, idxStr)
	}
	return
}

func buildMainPotHtml(g *Game) string {
	ongoing := g.ongoing
	html := `<div id="mainPot"></div>`
	mainPot := uint64(0)
	if ongoing != nil {
		mainPot = uint64(ongoing.mainPot.Get())
	}
	html += `<style>#mainPot:before { content: "Pot: ` + itoa1(mainPot) + `"; }</style>`
	return html
}

func buildMinRaiseHtml(g *Game) string {
	ongoing := g.ongoing
	html := `<div id="minRaise"></div>`
	minRaise := uint64(0)
	if ongoing != nil {
		minRaise = uint64(ongoing.minRaise.Get())
	}
	html += `<style>#minRaise:before { content: "Min raise: ` + itoa1(minRaise) + `"; }</style>`
	return html
}

func buildActionsDiv(roomID RoomID) (html string) {
	htmlTmpl := `
<table id="actionsDiv">
	<tr>
		<td>
			<iframe src="/poker/{{ .RoomID }}/deal" id="dealBtn"></iframe>
			<iframe src="/poker/{{ .RoomID }}/unsit" id="unSitBtn"></iframe>
		</td>
		<td style="vertical-align: top;">
			<iframe src="/poker/{{ .RoomID }}/bet" id="betBtn"></iframe>
		</td>
	</tr>
	<tr>
		<td></td>
		<td><div id="autoAction"></div></td>
	</tr>
	<tr>
		<td colspan="2"><div id="errorMsg"></div></td>
	</tr>
</table>`
	data := map[string]any{
		"RoomID": roomID.String(),
	}
	return simpleTmpl(htmlTmpl, data)
}

func simpleTmpl(htmlTmpl string, data any) string {
	var buf bytes.Buffer
	utils.Must1(utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data))
	return buf.String()
}

func buildSeatsHtml(g *Game, authUser *database.User) (html string) {
	g.Players.RWith(func(gPlayers seatedPlayers) {
		for i := range gPlayers {
			html += `<div id="seat` + itoa(i+1) + `Pot" class="seatPot"></div>`
		}
		html += `<div>`
		for i := range gPlayers {
			idxStr := itoa(i + 1)
			html += `<div class="seat" id="seat` + idxStr + `">`
			html += `	<div class="cash-bonus"></div>`
			html += `	<div class="throne"></div>`
			html += `   <iframe src="/poker/` + g.roomID.String() + `/sit/` + idxStr + `" class="takeSeat takeSeat` + idxStr + `"></iframe>`
			html += `	<div class="inner"></div>`
			html += `	<div id="seat` + idxStr + `_cash" class="cash"></div>`
			html += `	<div id="seat` + idxStr + `_status" class="status"></div>`
			html += `	<div id="countdown` + idxStr + `" class="countdown"><div class="progress-container"><div class="progress-bar animate"></div></div></div>`
			html += `</div>`
		}
		html += `</div>`
	})
	html += drawSeatsStyle(authUser, g)
	return html
}

func drawCashBonus(evt CashBonusEvent) (html string) {
	color := utils.Ternary(evt.IsGain, "#1ee91e", "orange")
	dur := utils.Ternary(evt.IsGain, "5s", "2s")
	fontSize := utils.Ternary(evt.IsGain, "25px", "18px")
	html += `<style>`
	html += fmt.Sprintf(`#seat%d .cash-bonus { animation: %s %s cubic-bezier(0.25, 0.1, 0.25, 1) forwards; color: %s; font-size: %s; }`,
		evt.PlayerSeatIdx+1, utils.Ternary(evt.Animation, "cashBonusAnimation", "cashBonusAnimation1"), dur, color, fontSize)
	html += fmt.Sprintf(`#seat%d .cash-bonus:before { content: "%s%s"; }`,
		evt.PlayerSeatIdx+1, utils.Ternary(evt.IsGain, "+", ""), evt.Amount)
	html += `</style>`
	return
}

func drawSeatsStyle(authUser *database.User, g *Game) string {
	ongoing := g.ongoing
	html := "<style>"
	seated := g.IsSeated(authUser.ID)
	g.Players.RWith(func(players seatedPlayers) {
		for i, p := range players {
			idxStr := itoa(i + 1)
			display := utils.Ternary(p != nil || seated, "none", "block")
			html += fmt.Sprintf(`.takeSeat%s { display: %s; }`, idxStr, display)
			if p != nil {
				pUserID := p.userID
				pUsername := p.username
				if pUserID == authUser.ID {
					html += fmt.Sprintf(`#seat%s { border: 2px solid #0d1b8f; }`, idxStr)
				}
				html += fmt.Sprintf(`#seat%s .inner:before { content: "%s"; }`, idxStr, pUsername.String())
				html += fmt.Sprintf(`#seat%s .throne { display: none; }`, idxStr)
				html += drawSeatCashLabel(idxStr, itoa2(p.getCash()))
				html += drawSeatStatusLabel(idxStr, p.getStatus())
				if ongoing != nil {
					if op := ongoing.players.get(pUserID); op != nil && op.GetBet() > 0 {
						html += drawSeatPotLabel(idxStr, itoa2(op.GetBet()))
					}
				}
			} else {
				html += fmt.Sprintf(`#seat%s { border: 1px solid #333; }`, idxStr)
				html += fmt.Sprintf(`#seat%s .inner:before { content: ""; }`, idxStr)
				html += fmt.Sprintf(`#seat%s .throne { display: block; }`, idxStr)
				html += drawSeatCashLabel(idxStr, "")
				html += drawSeatStatusLabel(idxStr, "")
			}
		}
	})
	html += "</style>"
	return html
}

func drawSeatPotLabel(seatIdxStr, betStr string) string {
	return fmt.Sprintf(`#seat%sPot:before { content: "%s"; }`, seatIdxStr, betStr)
}

func drawSeatCashLabel(seatIdxStr, cashStr string) string {
	return fmt.Sprintf(`#seat%s_cash:before { content: "%s"; }`, seatIdxStr, cashStr)
}

func drawSeatStatusLabel(seatIdxStr, statusStr string) string {
	return fmt.Sprintf(`#seat%s_status:before { content: "%s"; }`, seatIdxStr, statusStr)
}

func drawAutoActionMsgEvent(evt AutoActionEvent) (html string) {
	display := utils.Ternary(evt.Message != "", "block", "none")
	html += fmt.Sprintf(`<style>#autoAction { display: %s; } #autoAction:before { content: "%s"; }</style>`, display, evt.Message)
	return
}

func drawErrorMsgEvent(evt ErrorMsgEvent) (html string) {
	display := utils.Ternary(evt.Message != "", "block", "none")
	html += fmt.Sprintf(`<style>#errorMsg { display: %s; } #errorMsg:before { content: "%s"; }</style>`, display, evt.Message)
	return
}

func drawPlayerBetEvent(evt PlayerBetEvent) (html string) {
	idxStr := itoa(evt.PlayerSeatIdx + 1)
	html += `<style>`
	html += drawSeatPotLabel(idxStr, itoa2(evt.TotalBet))
	html += drawSeatCashLabel(idxStr, itoa2(evt.Cash))
	html += `</style>`
	return
}

func drawGameStartedEvent(evt GameStartedEvent, authUser *database.User) (html string) {
	pos := dealerTokenPos[evt.DealerSeatIdx]
	html += `<style>`
	html += `#dealerToken { top: ` + itoa(pos[0]) + `px; left: ` + itoa(pos[1]) + `px; }`
	html += `#dealBtn { visibility: hidden; }`
	html += `</style>`
	if authUser.PokerSoundsEnabled {
		html += `<audio src="/public/mp3/shuffle_cards.mp3" autoplay></audio>`
	}
	return
}

func buildWinnerHtml() string {
	html := `<div id="winner"></div>`
	html += `<style>#winner:before { content: ""; }</style>`
	return html
}

func drawGameIsDoneHtml(g *Game, evt GameIsDoneEvent) (html string) {
	html += `<style>`
	g.Players.RWith(func(gPlayers seatedPlayers) {
		for i, p := range gPlayers {
			if p != nil {
				html += drawSeatCashLabel(itoa(i+1), itoa2(p.getCash()))
			}
		}
	})
	html += `#winner:before { content: "Winner: ` + evt.Winner + ` (` + evt.WinnerHand + `)"; }`
	html += "</style>"
	return
}

func drawGameIsOverHtml(g *Game) (html string) {
	html += `<style>`
	html += `#dealBtn { visibility: visible; }`
	html += "</style>"
	return
}

func drawResetCardsEvent() (html string) {
	html += `<style>`
	for i := 1; i <= 52; i++ {
		idxStr := itoa(i)
		transition := fmt.Sprintf(` transition: %s ease-in-out; transform: translateX(%spx) translateY(%spx) rotateY(%s);`,
			animationTime.String(), itoa(DealerStackX), itoa(DealerStackY), BackfacingDeg)
		html += `#card` + idxStr + ` { z-index: ` + itoa(53-i) + `; ` + transition + ` }
				#card` + idxStr + ` .card .inner:before { content: ""; }`
	}
	html += `
				#winner:before { content: ""; }
				#mainPot:before { content: "Pot: 0"; }
			</style>`
	return
}

func drawPlayerFoldEvent(evt PlayerFoldEvent) (html string) {
	idx1Str := itoa(evt.Card1Idx)
	idx2Str := itoa(evt.Card2Idx)
	transition := fmt.Sprintf(`transition: %s ease-in-out; transform: translateX(%spx) translateY(%spx) rotateY(%s);`,
		animationTime.String(), itoa(BurnStackX), itoa(BurnStackY), BackfacingDeg)
	html = fmt.Sprintf(`<style>#card%s, #card%s { %s }</style>`, idx1Str, idx2Str, transition)
	return
}

func drawYourTurnHtml(authUser *database.User) (html string) {
	if authUser.PokerSoundsEnabled {
		html += `<audio src="/public/mp3/sound7.mp3" autoplay></audio>`
	}
	return
}

func drawCountDownStyle(evt PokerWaitTurnEvent) string {
	html := "<style>"
	html += hideCountdowns()
	html += resetSeatsBackgroundColor()
	remainingSecs := int((MaxUserCountdown*time.Second - time.Since(evt.CreatedAt)).Milliseconds())
	if evt.Idx >= 0 && evt.Idx <= 5 {
		idxStr := itoa(evt.Idx + 1)
		html += fmt.Sprintf(`#seat%s { background-color: rgba(200, 45, 45, 0.7); }`, idxStr)
		html += fmt.Sprintf(`#countdown%s { display: block; }`, idxStr)
		html += fmt.Sprintf(`#countdown%s .animate { --duration: %s; animation: progressBarAnimation calc(var(--duration) * 1ms) linear forwards; }`, idxStr, itoa(remainingSecs))
	}
	html += "</style>"
	return html
}

func createCssIDList(idFmt string) (out string) {
	cssIDList := make([]string, 0)
	for i := 1; i <= NbPlayers; i++ {
		cssIDList = append(cssIDList, fmt.Sprintf(idFmt, itoa(i)))
	}
	return strings.Join(cssIDList, ", ")
}

func hideCountdowns() (out string) {
	out += createCssIDList("#countdown%s") + ` { display: none; }`
	return
}

func resetSeatsBackgroundColor() (out string) {
	return createCssIDList("#seat%s") + ` { background-color: rgba(45, 45, 45, 0.4); }`
}

func resetSeatsPot() (out string) {
	return createCssIDList("#seat%sPot:before") + ` { content: ""; }`
}

func drawMainPotHtml(evt PokerMainPotUpdatedEvent) (html string) {
	html += `<style>`
	html += resetSeatsPot()
	html += `#mainPot:before { content: "Pot: ` + itoa2(evt.MainPot) + `"; }`
	html += `</style>`
	return
}

func drawMinRaiseHtml(evt PokerMinRaiseUpdatedEvent) (html string) {
	html += `<style>`
	html += `#minRaise:before { content: "Min raise: ` + itoa2(evt.MinRaise) + `"; }`
	html += `</style>`
	return
}

func getPokerEventHtml(payload PokerEvent, animationTime string) string {
	transform := `transform: translate(` + itoa(payload.Left) + `px, ` + itoa(payload.Top) + `px)`
	transform += utils.Ternary(payload.Angle != "", ` rotateZ(`+payload.Angle+`)`, ``)
	transform += utils.Ternary(!payload.Reveal, ` rotateY(`+BackfacingDeg+`)`, ``)
	transform += ";"
	pokerEvtHtml := `<style>`
	if payload.ID1 != 0 {
		pokerEvtHtml += `#card` + itoa(payload.ID1) + ` { z-index: ` + itoa(payload.ZIdx+1) + `; }`
	}
	pokerEvtHtml += `
#card` + itoa(payload.ID) + ` { z-index: ` + itoa(payload.ZIdx) + `; transition: ` + animationTime + ` ease-in-out; ` + transform + ` }
#card` + itoa(payload.ID) + ` .card .inner:before { content: "` + payload.Name + `"; color: ` + colorForCard(payload.Name) + `; }
</style>`
	return pokerEvtHtml
}

func buildDevHtml() (html string) {
	return `<div class="dev_seat1_card1"></div>
<div class="dev_seat2_card1"></div>
<div class="dev_seat3_card1"></div>
<div class="dev_seat4_card1"></div>
<div class="dev_seat5_card1"></div>
<div class="dev_seat6_card1"></div>
<div class="dev_community_card1"></div>
<div class="dev_community_card2"></div>
<div class="dev_community_card3"></div>
<div class="dev_community_card4"></div>
<div class="dev_community_card5"></div>
`
}

func buildHelpHtml() (html string) {
	html += `
<style>
.heart::after { content: '♥'; display: block; }
.diamond::after { content: '♥'; display: block; }
.spade::after { content: '♥'; display: block; }
.club::after { content: '♥'; display: block; }
.help { position: absolute; z-index: 999999; left: 50px; top: 12px; }
.help-content { display: none; }
.help:hover .help-content { display: block; }
.disabled::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: rgba(0, 0, 0, 0.4); /* Adjust the transparency as needed */
  pointer-events: none; /* Allow clicking through the overlay */
}
.title {
	font-family: Arial,Helvetica,sans-serif;
	font-weight: bolder;
}
</style>
<div class="help">
	Help
	<div class="help-content">
		<div style="position: absolute; top: 10px; left: 10px; padding: 10px; z-index: 1000; background-color: #ccc; display: flex; width: 365px; border: 1px solid black; border-radius: 5px;">
			<div style="margin-right: 20px">
				<div class="title">1- Royal Flush</div>
				<div>
					<div class="mini-card red">A<span class="heart"></span></div>
					<div class="mini-card red">K<span class="heart"></span></div>
					<div class="mini-card red">Q<span class="heart"></span></div>
					<div class="mini-card red">J<span class="heart"></span></div>
					<div class="mini-card red">10<span class="heart"></span></div>
				</div>
				<div class="title">2- Straight Flush</div>
				<div>
					<div class="mini-card red">10<span class="heart"></div>
					<div class="mini-card red">9<span class="heart"></div>
					<div class="mini-card red">8<span class="heart"></div>
					<div class="mini-card red">7<span class="heart"></div>
					<div class="mini-card red">6<span class="heart"></div>
				</div>
				<div class="title">3- Four of a kind</div>
				<div>
					<div class="mini-card red">A<span class="heart"></div>
					<div class="mini-card">A<span class="club"></div>
					<div class="mini-card red">A<span class="diamond"></div>
					<div class="mini-card">A<span class="spade"></div>
					<div class="mini-card red disabled">K<span class="heart"></div>
				</div>
				<div class="title">4- Full house</div>
				<div>
					<div class="mini-card red">A<span class="heart"></div>
					<div class="mini-card">A<span class="club"></div>
					<div class="mini-card red">A<span class="diamond"></div>
					<div class="mini-card">K<span class="spade"></div>
					<div class="mini-card red">K<span class="heart"></div>
				</div>
				<div class="title">5- Flush</div>
				<div>
					<div class="mini-card">K<span class="club"></div>
					<div class="mini-card">10<span class="club"></div>
					<div class="mini-card">8<span class="club"></div>
					<div class="mini-card">7<span class="club"></div>
					<div class="mini-card">5<span class="club"></div>
				</div>
			</div>
		
			<div>
				<div class="title">6- Straight</div>
				<div>
					<div class="mini-card red">10<span class="heart"></div>
					<div class="mini-card">9<span class="club"></div>
					<div class="mini-card red">8<span class="diamond"></div>
					<div class="mini-card">7<span class="spade"></div>
					<div class="mini-card red">6<span class="heart"></div>
				</div>
				<div class="title">7- Three of a kind</div>
				<div>
					<div class="mini-card red">A<span class="heart"></div>
					<div class="mini-card red">A<span class="diamond"></div>
					<div class="mini-card">A<span class="club"></div>
					<div class="mini-card disabled">K<span class="spade"></div>
					<div class="mini-card red disabled">Q<span class="heart"></div>
				</div>
				<div class="title">8- Two pair</div>
				<div>
					<div class="mini-card red">A<span class="heart"></div>
					<div class="mini-card">A<span class="club"></div>
					<div class="mini-card red">K<span class="diamond"></div>
					<div class="mini-card">K<span class="spade"></div>
					<div class="mini-card red disabled">7<span class="heart"></div>
				</div>
				<div class="title">9- Pair</div>
				<div>
					<div class="mini-card red">A<span class="heart"></div>
					<div class="mini-card">A<span class="club"></div>
					<div class="mini-card red disabled">K<span class="diamond"></div>
					<div class="mini-card disabled">J<span class="spade"></div>
					<div class="mini-card red disabled">7<span class="heart"></div>
				</div>
				<div class="title">10- High card</div>
				<div>
					<div class="mini-card red">A<span class="heart"></div>
					<div class="mini-card disabled">K<span class="club"></div>
					<div class="mini-card red disabled">Q<span class="diamond"></div>
					<div class="mini-card disabled">9<span class="spade"></div>
					<div class="mini-card red disabled">7<span class="heart"></div>
				</div>
			</div>
		</div>
	</div>
</div>`
	return
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func itoa1(i uint64) string {
	return fmt.Sprintf("%d", i)
}

func itoa2(i database.PokerChip) string {
	return fmt.Sprintf("%d", i)
}

var pokerCss = `<style>
html, body { height: 100%; width: 100%; }
body {
	background:linear-gradient(135deg, #449144 33%,#008a00 95%);
}
.card-holder{
	position: absolute;
	top: 0;
	left: 0;
	transform: translateX(` + itoa(DealerStackX) + `px) translateY(` + itoa(DealerStackY) + `px) rotateY(` + BackfacingDeg + `);
	transform-style: preserve-3d;
	backface-visibility: hidden;
	width:50px;
	height:70px;
	display:inline-block;
	box-shadow:1px 2px 2px rgba(0,0,0,.8);
	margin:2px;
}
.mini-card {
	width: 29px;
	height: 38px;
	padding-top: 2px;
	display: inline-grid;
	justify-content: center;
	justify-items: center;
	font-size: 20px;
	font-weight: bolder;
	background-color:#fcfcfc;
	border-radius:2%;
	border:1px solid black;
}
.red { color: #cc0000; }
.disabled { position: relative; background-color: #bbb; }
.card {
	box-shadow: inset 2px 2px 0 #fff, inset -2px -2px 0 #fff;
	transform-style: preserve-3d;
	position:absolute;
	top:0;
	left:0;
	bottom:0;
	right:0;
	backface-visibility: hidden;
	background-color:#fcfcfc;
	border-radius:2%;
	display:block;
	width:100%;
	height:100%;
	border:1px solid black;
}
.card .inner {
	padding: 5px;
	font-size: 25px;
	display: flex;
	justify-content: center;
}
.back{
	position:absolute;
	top:0;
	left:0;
	bottom:0;
	right:0;
	width:100%;
	height:100%;
	backface-visibility: hidden;
	transform: rotateY(` + BackfacingDeg + `);
	background: linear-gradient(135deg, #5D7B93 0%, #6D7E8C 50%, #4C6474 51%, #5D7B93 100%);
	border-radius:2%;
	box-shadow: inset 3px 3px 0 #fff, inset -3px -3px 0 #fff;
	display:block;
	border:1px solid black;
}
.back .inner {
	background-image: url(/public/img/trees.gif);
    width: 90%;
    height: 80%;
    background-size: contain;
    position: absolute;
    opacity: 0.4;
    margin-top: 8px;
    margin-left: 6px;
    background-repeat: no-repeat;
}
.takeSeat {
	width: 65px;
	height: 40px;
	display: flex;
	margin-left: auto;
	margin-right: auto;
	margin-top: 4px;
	position: absolute;
	left: 10px;
}
.seat {
	border: 1px solid #333;
	border-radius: 4px;
	background-color: rgba(45, 45, 45, 0.4);
	padding: 1px 2px;
	min-width: 80px;
	min-height: 48px;
	color: #ddd;
}
.seat .inner { display: flex; justify-content: center; }
.seat .cash { display: flex; justify-content: center; }
.seat .status { display: flex; justify-content: center; }
.seat .throne {
	background-image: url(/public/img/throne.png);
    background-size: contain;
    background-repeat: no-repeat;
    background-position: center;
    position: absolute;
    width: 100%;
    height: 90%;
    opacity: 0.3;
}

.dev_seat1_card1 { top: 55px; left: 610px; transform: rotateZ(-95deg); width:50px; height:70px; background-color: white; position: absolute; }
.dev_seat1_card2 {}
.dev_seat2_card1 { top: 175px; left: 620px; transform: rotateZ(-80deg); width:50px; height:70px; background-color: white; position: absolute; }
.dev_seat2_card2 {}
.dev_seat3_card1 { top: 290px; left: 580px; transform: rotateZ(-50deg); width:50px; height:70px; background-color: white; position: absolute; }
.dev_seat3_card2 {}
.dev_seat4_card1 { top: 310px; left: 430px; transform: rotateZ(0deg); width:50px; height:70px; background-color: white; position: absolute; }
.dev_seat4_card2 {}
.dev_seat5_card1 { top: 315px; left: 240px; transform: rotateZ(0deg); width:50px; height:70px; background-color: white; position: absolute; }
.dev_seat5_card2 {}
.dev_seat6_card1 { top: 270px; left: 70px; transform: rotateZ(10deg); width:50px; height:70px; background-color: white; position: absolute; }
.dev_seat6_card2 {}
.dev_community_card1 {top: ` + itoa(DealY) + `px; left: calc(` + itoa(DealX) + `px + 1 * ` + itoa(DealSpacing) + `px); width:50px; height:70px; background-color: white; position: absolute; }
.dev_community_card2 {top: ` + itoa(DealY) + `px; left: calc(` + itoa(DealX) + `px + 2 * ` + itoa(DealSpacing) + `px); width:50px; height:70px; background-color: white; position: absolute; }
.dev_community_card3 {top: ` + itoa(DealY) + `px; left: calc(` + itoa(DealX) + `px + 3 * ` + itoa(DealSpacing) + `px); width:50px; height:70px; background-color: white; position: absolute; }
.dev_community_card4 {top: ` + itoa(DealY) + `px; left: calc(` + itoa(DealX) + `px + 4 * ` + itoa(DealSpacing) + `px); width:50px; height:70px; background-color: white; position: absolute; }
.dev_community_card5 {top: ` + itoa(DealY) + `px; left: calc(` + itoa(DealX) + `px + 5 * ` + itoa(DealSpacing) + `px); width:50px; height:70px; background-color: white; position: absolute; }

#seat1 { position: absolute; top: 80px; left: 690px; }
#seat2 { position: absolute; top: 200px; left: 700px; }
#seat3 { position: absolute; top: 360px; left: 640px; }
#seat4 { position: absolute; top: 400px; left: 410px; }
#seat5 { position: absolute; top: 400px; left: 220px; }
#seat6 { position: absolute; top: 360px; left: 30px; }
#seat1_cash { }
#seat2_cash { }
#seat3_cash { }
#seat4_cash { }
#seat5_cash { }
#seat6_cash { }
.seatPot {
	font-size: 20px;
	font-family: Arial,Helvetica,sans-serif;
}
#seat1Pot { top: 88px; left: 528px; width: 50px; position: absolute; text-align: right; }
#seat2Pot { top: 190px; left: 530px; width: 50px; position: absolute; text-align: right; }
#seat3Pot { top: 280px; left: 525px; width: 50px; position: absolute; text-align: right; }
#seat4Pot { top: 290px; left: 430px; position: absolute; }
#seat5Pot { top: 290px; left: 240px; position: absolute; }
#seat6Pot { top: 245px; left: 86px; position: absolute; }
.takeSeat1 { }
.takeSeat2 { }
.takeSeat3 { }
.takeSeat4 { }
.takeSeat5 { }
.takeSeat6 { }
#actionsDiv { position: absolute; top: 470px; left: 100px; }
#dealBtn { width: 80px; height: 30px; display: inline-block; vertical-align: top; }
#unSitBtn { width: 80px; height: 30px; display: inline-block; vertical-align: top; }
#checkBtn { width: 60px; height: 30px; display: inline-block; vertical-align: top; }
#foldBtn { width: 50px; height: 30px; display: inline-block; vertical-align: top; }
#callBtn { width: 50px; height: 30px; display: inline-block; vertical-align: top; }
#betBtn { width: 400px; height: 45px; display: inline-block; vertical-align: top; }
.countdown { display: none; position: absolute; left: 0px; right: 2px; bottom: -9px; }
#mainPot { position: absolute; top: 220px; left: 215px; font-size: 20px; font-family: Arial,Helvetica,sans-serif; }
#minRaise { position: absolute; top: 220px; left: 365px; font-size: 18px; font-family: Arial,Helvetica,sans-serif; }
#winner { position: absolute; top: 265px; left: 250px; }
#errorMsg {
	margin-top: 10px;
	color: darkred;
	font-size: 20px;
	font-family: Arial,Helvetica,sans-serif;
	background-color: #ffadad;
	border: 1px solid #6e1616;
	padding: 2px 3px;
	border-radius: 3px;
	display: none;
}
#autoAction {
	color: #072a85;
	font-size: 20px;
	font-family: Arial,Helvetica,sans-serif;
	background-color: #bcd8ff;
	border: 1px solid #072a85;
	display: none;
	padding: 2px 3px;
	border-radius: 3px;
}
#chat-div { position: absolute; bottom: 0px; left: 0; right: 243px; min-width: 557px; height: 250px; z-index: 200; box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.3); }
#chat-top-bar { height: 57px; width: 100%; background-color: #222; }
#chat-content { height: 193px; width: 100%; background-color: #222; }
#eventLogs { position: absolute; bottom: 0px; right: 0px; width: 243px; height: 250px; background-color: #444; z-index: 200; box-shadow: 0px 0px 10px rgba(0, 0, 0, 0.3); }
#dealerToken { top: 142px; left: 714px; width: 20px; height: 20px; background-color: #ccc; border: 1px solid #333; border-radius: 11px; position: absolute; }
#dealerToken .inner { padding: 2px 4px; }
#dealerToken .inner:before { content: "D"; }
#soundsStatus {
	position: absolute; top: 10px; left: 10px;
}
#game {
	position: absolute;
	left: 0px;
	top: 0px;
	width: 760px;
	height: 400px;
}
#table {
	position: absolute; top: 20px; left: 20px; width: 750px; height: 400px; border-radius: 300px;
	background: radial-gradient(#449144, #008a00);
	box-shadow: rgba(0, 0, 0, 0.35) 0px 5px 15px;
	border: 5px solid #2c692c;
}
#table .inner {
	background-image: url(/public/img/trees.gif);
    width: 90%;
    height: 80%;
    display: block;
    position: absolute;
    background-size: contain;
    opacity: 0.07;
    background-repeat: no-repeat;
    background-position: right;
    margin-top: 45px;
}
#table .cards-outline {
    position: absolute;
    width: 280px;
    height: 80px;
    border: 3px solid rgba(128, 217, 133, 0.7);
    border-radius: 8px;
    left: 180px;
    top: 100px;
}

@keyframes cashBonusAnimation {
    0% {
        opacity: 1;
        transform: translateY(0);
    }
    66.66% {
        opacity: 1;
        transform: translateY(0);
		transform: translateY(-15px);
    }
    100% {
        opacity: 0;
        transform: translateY(-30px); /* Adjust the distance it moves up */
    }
}

@keyframes cashBonusAnimation1 {
    0% {
        opacity: 1;
        transform: translateY(0);
    }
    66.66% {
        opacity: 1;
        transform: translateY(0);
		transform: translateY(-15px);
    }
    100% {
        opacity: 0;
        transform: translateY(-30px); /* Adjust the distance it moves up */
    }
}

.cash-bonus {
	z-index: 108;
    position: absolute;
	background-color: rgba(0, 0, 0, 0.99);
	padding: 1px 5px;
	border-radius: 5px;
	opacity: 0;
	font-family: Arial,Helvetica,sans-serif;
}

/* Styles for the progress bar container */
.progress-container {
  width: 100%;
  height: 6px;
  background-color: #f0f0f0;
  overflow: hidden;
  box-shadow: inset 0 0 5px rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(0, 0, 0, 0.9);
}
.progress-bar {
  height: 100%;
  width: 100%;
  background-color: #4caf50;
}
@keyframes progressBarAnimation {
  from { width: 100%; }
  to   { width: 0;    }
}

</style>`
