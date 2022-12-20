package v1

import (
	"bytes"
	"context"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/hashset"
	"dkforest/pkg/utils"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"html/template"
	"math/rand"
	"sort"
	"strings"
	"time"
)

var WWInstance *Werewolf

const (
	PreGameState = iota + 1
	DayState
	NightState
	VoteState
	EndGameState
)

const (
	TownspeopleRole = "townspeople"
	WerewolfRole    = "werewolf"
	SeerRole        = "seer"
	HealerRole      = "healer"
)

var ErrInvalidPlayerName = errors.New("unknown player name, please send a valid name")

type Werewolf struct {
	ctx              context.Context
	cancel           context.CancelFunc
	readyCh          chan bool
	narratorID       database.UserID
	roomID           database.RoomID
	werewolfGroupID  database.GroupID
	spectatorGroupID database.GroupID
	deadGroupID      database.GroupID
	players          map[string]*Player
	playersAlive     map[string]*Player
	spectators       map[database.UserID]struct{}
	werewolves       []int64
	state            int64
	werewolfSet      *hashset.HashSet[database.UserID]
	spectatorSet     *hashset.HashSet[database.UserID]
	healerID         *database.UserID
	seerID           *database.UserID
	townspersonSet   *hashset.HashSet[database.UserID]
	werewolfCh       chan string
	seerCh           chan string
	healerCh         chan string
	votesCh          chan string
	voted            *hashset.HashSet[database.UserID] // Keep track of which user voted already
}

// Return either or not the userID is an active player (alive)
func (b *Werewolf) isAlivePlayer(userID database.UserID) bool {
	for _, player := range b.playersAlive {
		if player.UserID == userID {
			return true
		}
	}
	return false
}

func (b *Werewolf) InterceptPreGameMsg(cmd *Command) {
	if cmd.message == "/players" {
		b.Narrate("Registered players: "+b.alivePlayersStr(), nil, nil)
		cmd.err = ErrRedirect
		return

	} else if cmd.message == "/join" {
		if cmd.authUser.IsHellbanned {
			cmd.err = ErrRedirect
			return
		}
		if _, found := b.players[cmd.authUser.Username]; found {
			cmd.err = ErrRedirect
			return
		}
		player := &Player{
			UserID:   cmd.authUser.ID,
			Username: cmd.authUser.Username,
		}
		b.players[cmd.authUser.Username] = player
		b.playersAlive[cmd.authUser.Username] = player
		b.Narrate("@"+cmd.authUser.Username+" joined the Game", nil, nil)
		cmd.err = ErrRedirect
		return

	} else if cmd.message == "/spectate" {
		b.spectators[cmd.authUser.ID] = struct{}{}
		b.Narrate("@"+cmd.authUser.Username+" spectate the Game", nil, nil)
		cmd.err = ErrRedirect
		return

	} else if cmd.message == "/start" {
		b.cancel()
		time.Sleep(time.Second)
		utils.SGo(func() {
			b.StartGame()
		})
		cmd.err = ErrRedirect
		return
	}
}

func (b *Werewolf) InterceptNightMsg(cmd *Command) {
	if cmd.groupID != nil && *cmd.groupID == b.werewolfGroupID {
		select {
		case b.werewolfCh <- cmd.message:
			cmd.err = ErrRedirect
		default:
			cmd.err = errors.New("narrator doesn't need your input")
		}
		return
	} else if b.isForNarrator(cmd) && b.seerID != nil && cmd.fromUserID == *b.seerID {
		select {
		case b.seerCh <- cmd.message:
			cmd.err = ErrRedirect
		default:
			cmd.err = errors.New("narrator doesn't need your input")
		}
		return
	} else if b.isForNarrator(cmd) && b.healerID != nil && cmd.fromUserID == *b.healerID {
		select {
		case b.healerCh <- cmd.message:
			cmd.err = ErrRedirect
		default:
			cmd.err = errors.New("narrator doesn't need your input")
		}
		return
	}
	cmd.err = errors.New("chat disabled")
	return
}

// Return either or not the message is a PM for the narrator
func (b *Werewolf) isForNarrator(cmd *Command) bool {
	return cmd.toUser != nil && cmd.toUser.ID == b.narratorID
}

func (b *Werewolf) InterceptVoteMsg(cmd *Command) {
	if !b.isAlivePlayer(cmd.fromUserID) || !b.isForNarrator(cmd) {
		cmd.err = errors.New("chat disabled")
		return
	}
	if b.isForNarrator(cmd) {
		if !b.voted.Contains(cmd.fromUserID) {
			name := cmd.message
			if b.isValidPlayerName(name) {
				b.votesCh <- name
			} else {
				b.Narrate(ErrInvalidPlayerName.Error(), &cmd.fromUserID, nil)
			}
		} else {
			b.Narrate("You have already voted", &cmd.fromUserID, nil)
		}
	}
}

var tuto = `Tutorial:
"/join" to join the Game
"/players" list the players that have joined the Game
"/start" to start the Game
"/stop" to stop the Game
"/ready" will skip the 5min conversation
"/tuto" will display this tutorial
"/clear" will reset the room and display this tutorial

Werewolf: To kill someone during the night, you have to reply in the "werewolf" group with the name of the person to kill (no @)
Seer/Healer: You have reply to the narrator with the name (eg: "/pm 0 n0tr1v")
Townspeople: To vote, you have to pm the narrator with a name (eg: "/pm 0 n0tr1v")`

func (b *Werewolf) InterceptMsg(cmd *Command) {
	if cmd.room.ID != b.roomID {
		return
	}

	SlashInterceptor{}.InterceptMsg(cmd)

	// If the message is a PM not for the narrator, we reject it
	if cmd.toUser != nil && (cmd.toUser.ID != b.narratorID && cmd.fromUserID != b.narratorID) {
		cmd.err = errors.New("PM not allowed at this room")
		return
	}

	// Spectator can chat all the time
	if cmd.groupID != nil && *cmd.groupID == b.spectatorGroupID {
		return
	}

	if cmd.authUser.IsModerator() && cmd.message == "/stop" {
		b.cancel()
		cmd.err = ErrRedirect
		return
	} else if cmd.authUser.IsModerator() && cmd.message == "/ready" {
		b.readyCh <- true
		cmd.err = ErrRedirect
		return
	} else if cmd.authUser.IsModerator() && cmd.message == "/tuto" {
		b.Narrate(tuto, nil, nil)
		cmd.err = ErrRedirect
		return
	} else if cmd.authUser.IsModerator() && cmd.message == "/clear" {
		_ = database.DeleteChatRoomMessages(b.roomID)
		b.Narrate(tuto, nil, nil)
		cmd.err = ErrRedirect
		return
	}

	// Anyone can talk during these states
	if b.state == PreGameState || b.state == EndGameState {
		if b.state == PreGameState {
			b.InterceptPreGameMsg(cmd)
		}
		return
	}

	// Otherwise, non-playing people cannot talk in public chat
	if !b.isAlivePlayer(cmd.authUser.ID) {
		cmd.err = errors.New("public chat disabled")
		return
	}

	switch b.state {
	case DayState:
	case VoteState:
		b.InterceptVoteMsg(cmd)
	case NightState:
		b.InterceptNightMsg(cmd)
	default:
		cmd.err = errors.New("public chat disabled")
		return
	}
}

// Wait until we receive the votes from all the players
func (b *Werewolf) waitVotes() (votes []string) {
	for len(votes) < len(b.playersAlive) {
		var vote string
		select {
		case vote = <-b.votesCh:
		case <-time.After(15 * time.Second):
			b.Narrate(fmt.Sprintf("Waiting votes %d/%d", len(votes), len(b.playersAlive)), nil, nil)
			continue
		case <-b.ctx.Done():
			return
		}
		votes = append(votes, vote)
	}
	return
}

func (b *Werewolf) waitNameFromWerewolf() (name string) {
	for {
		select {
		case name = <-b.werewolfCh:
		case <-time.After(15 * time.Second):
			b.Narrate("Waiting reply from werewolf", nil, nil)
			continue
		case <-b.ctx.Done():
			return
		}
		if b.isValidPlayerName(name) {
			break
		}
		b.Narrate(ErrInvalidPlayerName.Error(), nil, &b.werewolfGroupID)
	}
	return name
}

func (b *Werewolf) waitNameFromSeer() (name string) {
	for {
		select {
		case name = <-b.seerCh:
		case <-time.After(15 * time.Second):
			b.Narrate("Waiting reply from seer", nil, nil)
			continue
		case <-b.ctx.Done():
			return
		}
		if b.isValidPlayerName(name) {
			break
		}
		b.Narrate(ErrInvalidPlayerName.Error(), b.seerID, nil)
	}
	return name
}

func (b *Werewolf) waitNameFromHealer() (name string) {
	for {
		select {
		case name = <-b.healerCh:
		case <-time.After(15 * time.Second):
			b.Narrate("Waiting reply from healer", nil, nil)
			continue
		case <-b.ctx.Done():
			return
		}
		if b.isValidPlayerName(name) {
			break
		}
		b.Narrate(ErrInvalidPlayerName.Error(), b.healerID, nil)
	}
	return name
}

// Return either a name is a valid alive player name or not
func (b *Werewolf) isValidPlayerName(name string) bool {
	name = strings.TrimSpace(name)
	for _, player := range b.playersAlive {
		if player.Username == name {
			return true
		}
	}
	return false
}

// Narrate register a chat message on behalf of the narrator user
func (b *Werewolf) Narrate(msg string, toUserID *database.UserID, groupID *database.GroupID) {
	html, _ := ProcessRawMessage(msg, "", b.narratorID, b.roomID, nil)
	b.NarrateRaw(html, toUserID, groupID)
}

func (b *Werewolf) NarrateRaw(msg string, toUserID *database.UserID, groupID *database.GroupID) {
	_, _ = database.CreateOrEditMessage(nil, msg, msg, "", b.roomID, b.narratorID, toUserID, nil, groupID, false, false, false)
}

// Display roles assigned at beginning of the Game
func (b *Werewolf) displayRoles() {
	msg := "Roles were:\n"
	for _, player := range b.players {
		msg += "@" + player.Username + " : " + player.Role + "\n"
	}
	b.Narrate(msg, nil, nil)
}

func (b *Werewolf) StartGame() {
	defer func() {
		b.displayRoles()
		b.reset()
	}()
	b.ctx, b.cancel = context.WithCancel(context.Background())
	// Assign roles
	playersArr := make([]*Player, 0)
	for _, player := range b.playersAlive {
		playersArr = append(playersArr, player)
	}
	rand.Shuffle(len(playersArr), func(i, j int) { playersArr[i], playersArr[j] = playersArr[j], playersArr[i] })
	for idx, player := range playersArr {
		if idx == 0 {
			b.werewolfSet.Insert(player.UserID)
			_, _ = database.AddUserToRoomGroup(b.roomID, b.werewolfGroupID, player.UserID)
			player.Role = WerewolfRole
			werewolfMsg := "During the day you seem to be a regular Townsperson.\n" +
				"However, you’ve been kissed by the Night and transform into a Werewolf when the sun sets.\n" +
				"Your new nature compels you to kill and eat a Townsperson every night."
			b.Narrate(werewolfMsg, &player.UserID, nil)
		} else if idx == 1 {
			b.townspersonSet.Insert(player.UserID)
			b.healerID = &player.UserID
			player.Role = HealerRole
			healerMsg := "You’re a Townsperson with the unique ability to save lives.\n" +
				"During the night, you’ll get a chance to protect another Townsperson from death if they are attacked by the Werewolves.\n" +
				"You can choose to protect yourself."
			b.Narrate(healerMsg, &player.UserID, nil)
		} else if idx == 2 {
			b.townspersonSet.Insert(player.UserID)
			b.seerID = &player.UserID
			player.Role = SeerRole
			seerMsg := "You’re a Townsperson with the unique ability to peer into a person’s soul and see their true nature.\n" +
				"During the night, you’ll get a chance to see if another Townsperson is a Werewolf.\n" +
				"However, use this information wisely because it can lead to you being targeted by the Werewolves the next night if they deduce your identity."
			b.Narrate(seerMsg, &player.UserID, nil)
		} else {
			b.townspersonSet.Insert(player.UserID)
			player.Role = TownspeopleRole
			townspersonMsg := "You’re a regular member of the town.\n" +
				"Perhaps you’re a baker, merchant, or soldier.\n" +
				"Your job is to save the town by eliminating the Werewolves that have infiltrated your town and started feeding on your neighbors.\n" +
				"Also, try to avoid getting killed yourself."
			b.Narrate(townspersonMsg, &player.UserID, nil)
		}
	}
	b.state = DayState
	b.Narrate("Players: "+b.alivePlayersStr(), nil, nil)
	b.Narrate("Day 1: It is day time. Players can now introduce themselves. (5min)", nil, nil)

	select {
	case <-time.After(5 * time.Minute):
	case <-b.readyCh:
	case <-b.ctx.Done():
		b.Narrate("STOP SIGNAL - Game is being stopped", nil, nil)
		return
	}

	for {
		b.state = NightState
		b.Narrate("Townspeople, go to sleep", nil, nil)
		playerNameToKill := b.processWerewolf()
		b.processSeer()
		playerNameToSave := b.processHealer()

		b.state = DayState
		b.Narrate("Townspeople, wake up", nil, nil)
		if playerNameToKill == playerNameToSave {
			b.Narrate("Someone was attacked last night, but they survived", nil, nil)
		} else {
			b.Narrate("Everyone wakes up to see a trail of blood leading to the forest.\n"+
				"There you find @"+playerNameToKill+"’s mangled remains by the Great Oak.\n"+
				"Curiously, there are deep claw marks in the bark of the surrounding trees.\n"+
				"It looks like @"+playerNameToKill+" put up a fight.", nil, nil)
			b.kill(playerNameToKill)
		}

		b.Narrate("Players still alive: "+b.alivePlayersStr(), nil, nil)
		if b.werewolfSet.Len() == 0 {
			b.Narrate("Townspeople win", nil, nil)
			break
		} else if b.townspersonSet.Len() == 1 {
			b.Narrate("Werewolf win", nil, nil)
			break
		}

		b.Narrate("Townspeople now have 5min to discuss the events", nil, nil)

		select {
		case <-time.After(5 * time.Minute):
		case <-b.readyCh:
		case <-b.ctx.Done():
			b.Narrate("STOP SIGNAL - Game is being stopped", nil, nil)
			return
		}

		b.state = VoteState
		b.voted = hashset.New[database.UserID]()
		b.Narrate("It's now time to vote for execution. PM me the name you vote to execute or \"none\"", nil, nil)
		killName := b.killVote()
		if killName == "" {
			b.Narrate("Townspeople do not want to execute anyone", nil, nil)
		} else {
			b.Narrate("Townspeople execute @"+killName, nil, nil)
			b.kill(killName)
		}

		b.Narrate("Players still alive: "+b.alivePlayersStr(), nil, nil)

		if b.werewolfSet.Len() == 0 {
			b.Narrate("Townspeople win", nil, nil)
			break
		} else if b.townspersonSet.Len() == 1 {
			b.Narrate("Werewolf win", nil, nil)
			break
		}
	}
	b.state = EndGameState
	b.Narrate("Game ended", nil, nil)
}

// Return the names of alive players. ie: "user1, user2, user3"
func (b *Werewolf) alivePlayersStr() (out string) {
	arr := make([]string, 0)
	for _, player := range b.playersAlive {
		arr = append(arr, "@"+player.Username)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	return strings.Join(arr, ", ")
}

// Kill a player
func (b *Werewolf) kill(playerName string) {
	player, found := b.playersAlive[playerName]
	if !found {
		return
	}
	delete(b.playersAlive, playerName)
	switch player.Role {
	case WerewolfRole:
		b.werewolfSet.Remove(player.UserID)
		_ = database.RmUserFromRoomGroup(b.roomID, b.werewolfGroupID, player.UserID)
	case TownspeopleRole:
		b.townspersonSet.Remove(player.UserID)
	case HealerRole:
		b.townspersonSet.Remove(player.UserID)
		b.healerID = nil
	case SeerRole:
		b.townspersonSet.Remove(player.UserID)
		b.seerID = nil
	}
	_, _ = database.AddUserToRoomGroup(b.roomID, b.deadGroupID, player.UserID)
}

// Return the name of the player name that receive the most vote
func (b *Werewolf) killVote() string {

	// Send a PM to all players saying they have to vote for a name
	for _, player := range b.playersAlive {
		msg := "Who do you vote to kill? (name | none)<br />"
		b.createKillVoteForm()
		b.Narrate(msg, &player.UserID, nil)
	}

	votes := b.waitVotes()
	// Get the max voted name
	maxName := "none"
	maxCount := 0
	voteMap := make(map[string]int) // keep track of how many votes for each values
	for _, vote := range votes {
		tmp := voteMap[vote]
		tmp++
		voteMap[vote] = tmp
		if tmp > maxCount {
			maxCount = tmp
			maxName = vote
		}
	}
	if maxName == "none" {
		return ""
	}
	return maxName
}

func (b *Werewolf) getAlivePlayersArr(includeWerewolves bool) []string {
	arr := make([]string, 0)
	for _, player := range b.playersAlive {
		if !includeWerewolves && b.werewolfSet.Contains(player.UserID) {
			continue
		}
		arr = append(arr, player.Username)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	return arr
}

func (b *Werewolf) createPickUserForm() string {
	arr := b.getAlivePlayersArr(true)

	htmlTmpl := `
<form method="post" action="/api/v1/chat/top-bar/werewolf" target="iframe1">
	{{ range $idx, $p := .Arr }}
		<input type="radio" ID="player{{ $idx }}" name="message" value="/pm 0 {{ $p }}" /><label for="player{{ $idx }}">{{ $p }}</label><br />
	{{ end }}
	<button type="submit" name="btn_submit">ok</button>
</form>`
	data := map[string]any{
		"Arr": arr,
	}
	var buf bytes.Buffer
	_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (b *Werewolf) createKillVoteForm() string {
	arr := b.getAlivePlayersArr(true)

	htmlTmpl := `
<form method="post" action="/api/v1/chat/top-bar/werewolf" target="iframe1">
	{{ range $idx, $p := .Arr }}
		<input type="radio" ID="player{{ $idx }}" name="message" value="/pm 0 {{ $p }}" /><label for="player{{ $idx }}">{{ $p }}</label><br />
	{{ end }}
	<input type="radio" ID="none" name="message" value="/pm 0 none" /><label for="none">none</label><br />
	<button type="submit" name="btn_submit">ok</button>
</form>`
	data := map[string]any{
		"Arr": arr,
	}
	var buf bytes.Buffer
	_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (b *Werewolf) createWerewolfPickUserForm() string {
	arr := b.getAlivePlayersArr(false)

	htmlTmpl := `
<form method="post" action="/api/v1/chat/top-bar/werewolf" target="iframe1">
	{{ range $idx, $p := .Arr }}
		<input type="radio" ID="player{{ $idx }}" name="message" value="/g werewolf {{ $p }}" /><label for="player{{ $idx }}">{{ $p }}</label><br />
	{{ end }}
	<button type="submit" name="btn_submit">ok</button>
</form>`
	data := map[string]any{
		"Arr": arr,
	}
	var buf bytes.Buffer
	_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (b *Werewolf) processWerewolf() string {
	b.UnlockGroup("werewolf")
	msg := "Werewolf, who do you to kill?"
	msg += b.createWerewolfPickUserForm()
	b.NarrateRaw(msg, nil, &b.werewolfGroupID)
	name := b.waitNameFromWerewolf()
	b.Narrate(name+" will be killed", nil, &b.werewolfGroupID)
	b.LockGroup("werewolf")
	return name
}

func (b *Werewolf) processSeer() {
	if b.seerID == nil {
		return
	}
	arr := make([]string, 0)
	for _, player := range b.playersAlive {
		arr = append(arr, player.Username)
	}
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	msg := "Seer, who do you want to identify?<br />"
	msg += b.createPickUserForm()
	b.NarrateRaw(msg, b.seerID, nil)
	name := b.waitNameFromSeer()
	player := b.playersAlive[name]
	b.Narrate(name+" is a "+player.Role, b.seerID, nil)
}

func (b *Werewolf) processHealer() string {
	if b.healerID == nil {
		return ""
	}
	msg := "Healer, who do you want to save?"
	msg += b.createPickUserForm()
	b.NarrateRaw(msg, b.healerID, nil)
	name := b.waitNameFromHealer()
	b.Narrate(name+" will survive the night", b.healerID, nil)
	return name
}

func (b *Werewolf) LockGroups() {
	b.LockGroup("werewolf")
}

func (b *Werewolf) LockGroup(groupName string) {
	group, _ := database.GetRoomGroupByName(b.roomID, groupName)
	group.Locked = true
	group.DoSave()
}

func (b *Werewolf) UnlockGroup(groupName string) {
	group, _ := database.GetRoomGroupByName(b.roomID, groupName)
	group.Locked = false
	group.DoSave()
}

type Player struct {
	UserID   database.UserID
	Username string
	Role     string
}

func (b *Werewolf) reset() {
	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.state = PreGameState
	b.spectators = make(map[database.UserID]struct{})
	b.players = make(map[string]*Player)
	b.playersAlive = make(map[string]*Player)
	b.werewolfSet = hashset.New[database.UserID]()
	b.spectatorSet = hashset.New[database.UserID]()
	b.townspersonSet = hashset.New[database.UserID]()
	b.voted = hashset.New[database.UserID]()
	b.werewolfCh = make(chan string)
	b.seerCh = make(chan string)
	b.healerCh = make(chan string)
	b.votesCh = make(chan string)
	b.readyCh = make(chan bool)
}

func NewWerewolf() *Werewolf {
	// Prepare room
	room, err := database.GetChatRoomByName("werewolf")
	if err != nil {
		logrus.Error("#werewolf room not found")
		return nil
	}
	zeroUser, _ := database.GetUserByUsername(config.NullUsername)
	_ = database.DeleteChatRoomGroups(room.ID)
	werewolfGroup, _ := database.CreateChatRoomGroup(room.ID, "werewolf", "#ffffff")
	werewolfGroup.Locked = true
	werewolfGroup.DoSave()
	spectatorGroup, _ := database.CreateChatRoomGroup(room.ID, "spectator", "#ffffff")
	deadGroup, _ := database.CreateChatRoomGroup(room.ID, "dead", "#ffffff")

	b := new(Werewolf)
	b.werewolfGroupID = werewolfGroup.ID
	b.spectatorGroupID = spectatorGroup.ID
	b.deadGroupID = deadGroup.ID
	b.narratorID = zeroUser.ID
	b.roomID = room.ID
	b.reset()
	return b
}
