package interceptors

import (
	"bytes"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fogleman/gg"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/notnil/chess"
	"github.com/notnil/chess/uci"
	"github.com/sirupsen/logrus"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ChessMove struct {
	IDStr1     string
	IDStr2     string
	EnPassant  string
	CheckIDStr string
	Move       chess.Move
	MoveIdx    int
	BestMove   string
}

type ChessAnalyzeProgress struct {
	Step  int
	Total int
}

var ChessAnalyzeProgressPubSub = pubsub.NewPubSub[ChessAnalyzeProgress]()
var ChessPubSub = pubsub.NewPubSub[ChessMove]()

type ChessPlayer struct {
	ID              database.UserID
	Username        database.Username
	UserStyle       string
	NotifyChessMove bool
}

type ChessGame struct {
	DbChessGame    *database.ChessGame
	Key            string
	Game           *chess.Game
	lastUpdated    time.Time
	Player1        *ChessPlayer
	Player2        *ChessPlayer
	CreatedAt      time.Time
	piecesCache    map[chess.Square]string
	analyzing      bool
	mtx            sync.RWMutex
	analyzeProgrss ChessAnalyzeProgress
}

func (g *ChessGame) SetAnalyzeProgress(progress ChessAnalyzeProgress) {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	g.analyzeProgrss = progress
}

func (g *ChessGame) GetAnalyzeProgress() ChessAnalyzeProgress {
	g.mtx.RLock()
	defer g.mtx.RUnlock()
	return g.analyzeProgrss
}

func newChessPlayer(player database.User) *ChessPlayer {
	p := new(ChessPlayer)
	p.ID = player.ID
	p.Username = player.Username
	p.UserStyle = player.GenerateChatStyle()
	p.NotifyChessMove = player.NotifyChessMove
	return p
}

func newChessGame(gameKey string, player1, player2 database.User, dbChessGame *database.ChessGame) *ChessGame {
	g := new(ChessGame)
	g.DbChessGame = dbChessGame
	g.CreatedAt = time.Now()
	g.Key = gameKey
	g.Game = chess.NewGame()
	if dbChessGame.PGN != "" {
		pgnOpt, _ := chess.PGN(strings.NewReader(dbChessGame.PGN))
		g.Game = chess.NewGame(pgnOpt)
	}
	g.lastUpdated = time.Now()
	g.Player1 = newChessPlayer(player1)
	g.Player2 = newChessPlayer(player2)
	g.piecesCache = InitPiecesCache(g.Game.Moves())
	return g
}

type Chess struct {
	sync.Mutex
	db     *database.DkfDB
	zeroID database.UserID
	games  map[string]*ChessGame
}

func NewChess(db *database.DkfDB) *Chess {
	zeroUser, _ := db.GetUserByUsername(config.NullUsername)
	c := &Chess{db: db, zeroID: zeroUser.ID}
	c.games = make(map[string]*ChessGame)

	// Thread that cleanup inactive games
	go func() {
		for {
			time.Sleep(15 * time.Minute)
			c.Lock()
			for k, g := range c.games {
				if time.Since(g.lastUpdated) > 3*time.Hour {
					delete(c.games, k)
				}
			}
			c.Unlock()
		}
	}()

	return c
}

var ChessInstance *Chess

const (
	sqSize    = 45
	boardSize = 8 * sqSize

	CheckColor    = "rgba(255, 0, 0, 0.4)"
	LastMoveColor = "rgba(0, 255, 0, 0.2)"
)

func GetID(row, col int, isFlipped bool) (id int) {
	if isFlipped {
		id = row*8 + (7 - col)
	} else {
		id = (7-row)*8 + col
	}
	return id
}

var ChessCSS = `
<style>
#arrow {
	transform-origin: top center !important;
	display: none;
	position: absolute;
	top: 0;
	left: 0;
	width: 12.5%;
	height: 12.5%;
	z-index: 4;
	pointer-events: none;
}
#arrow .triangle-up {
	position: absolute;
	width: 60%;
	height: 45%;
	left: 20%;
	background: rgba(0, 0, 255, 0.6);
	clip-path: polygon(0% 100%, 50% 0%, 100% 100%);
}
#arrow .rectangle {
	position: absolute;
	top: 45%;
	left: 42.5%;
	width: 15%;
	height: 55%;
	background-color: rgba(0, 0, 255, 0.6);
	border-radius: 0 0 10px 10px
}
.newBoard {
	position: relative;
	aspect-ratio: 1 / 1;
	width: 100%;
	min-height: 360px;
}
.newBoard .img {
	position: absolute;
	width: 12.5%;
	height: 12.5%;
	background-size: 100%;
}
label {
	position: absolute;
	width: 12.5%;
	height: 12.5%;
}
input[type=checkbox] {
    display:none;
}
input[type=checkbox] + label {
    display: inline-block;
    padding: 0 0 0 0;
	margin: 0 0 0 0;
    background-size: 100%;
	border: 3px solid transparent;
	box-sizing: border-box;
}
input[type=checkbox]:checked + label {
    display: inline-block;
    background-size: 100%;
	border: 3px solid red;
}
</style>`

func (g *ChessGame) renderBoardHTML1(moveIdx int, position *chess.Position, isFlipped bool, imgB64 string, bestMove *chess.Move) string {
	game := g.Game
	moves := game.Moves()
	var last *chess.Move
	if len(moves) > 0 {
		last = moves[len(moves)-1]
		if moveIdx > 0 && moveIdx < len(moves) {
			last = moves[moveIdx-1]
		}
	}
	deadBoardMap := chess.NewGame().Position().Board().SquareMap()

	pieceInCheck := func(p chess.Piece) bool {
		return last != nil && p.Color() == position.Turn() && p.Type() == chess.King && last.HasTag(chess.Check)
	}
	sqIsBestMove := func(sq chess.Square) bool {
		return bestMove != nil && (bestMove.S1() == sq || bestMove.S2() == sq)
	}
	sqIsLastMove := func(sq chess.Square) bool {
		return last != nil && (last.S1() == sq || last.S2() == sq)
	}
	getPieceFileName := func(p chess.Piece) string {
		return "/public/img/chess/" + p.Color().String() + strings.ToUpper(p.Type().String()) + ".png"
	}

	htmlTmpl := ChessCSS + `
<table class="newBoard" style="	background-repeat: no-repeat; background-size: cover; background-image: url(data:image/png;base64,{{ .ImgB64 }}); overflow: hidden;">
	{{ range $row := .Rows }}
		<tr>
			{{ range $col := $.Cols }}
				{{ $id := GetID $row $col }}
				{{ $sq := Square $id }}
				{{ $pidStr := GetPid $sq }}
				<td class="square square_{{ $id }}" style="background-color: {{ if IsBestMove $sq }}rgba(0, 0, 255, 0.2){{ else if IsLastMove $sq }}{{ $.LastMoveColor | css }}{{ else }}transparent{{ end }};">
					{{ if and (eq $col 0) (eq $row 0) }}
						<div id="arrow"><div class="triangle-up"></div><div class="rectangle"></div></div>
					{{ end }}
					{{ if $pidStr }}
						{{ $p := PieceFromSq $sq }}
						<div id="{{ $pidStr }}" class="img" style=" display: none; background-image: url({{ GetPieceFileName $p }});"></div>
					{{ end }}
				</td>
			{{ end }}
		</tr>
	{{ end }}
</table>
<style>
{{- range $row := .Rows -}}
	{{ range $col := $.Cols -}}
		{{- $id := GetID $row $col -}}
		{{- $sq := Square $id -}}
		{{- $p := PieceFromSq1 $sq -}}
		{{- $pidStr := GetPid1 $sq -}}
		{{- if $pidStr -}}
			#{{ $pidStr }} {
				display: block !important;
				background-image: url("{{ GetPieceFileName $p }}") !important;
				left: calc({{ $col }}*12.5%); top: calc({{ $row }}*12.5%);
				background-color: {{ if PieceInCheck $p }}{{ $.CheckColor | css }}{{ else }}transparent{{ end }};
			}
		{{- end -}}
	{{- end -}}
{{- end -}}
</style>
`

	allPieces := []chess.Square{
		chess.A8, chess.B8, chess.C8, chess.D8, chess.E8, chess.F8, chess.G8, chess.H8,
		chess.A7, chess.B7, chess.C7, chess.D7, chess.E7, chess.F7, chess.G7, chess.H7,
		chess.A2, chess.B2, chess.C2, chess.D2, chess.E2, chess.F2, chess.G2, chess.H2,
		chess.A1, chess.B1, chess.C1, chess.D1, chess.E1, chess.F1, chess.G1, chess.H1,
	}
	dead := make([]chess.Square, 0)
	for _, p := range allPieces {
		found := false
		for _, v := range g.piecesCache {
			if v == "piece_"+p.String() {
				found = true
				break
			}
		}
		if !found {
			dead = append(dead, p)
		}
	}

	data := map[string]any{
		"ImgB64":        imgB64,
		"Rows":          []int{0, 1, 2, 3, 4, 5, 6, 7},
		"Cols":          []int{0, 1, 2, 3, 4, 5, 6, 7},
		"LastMoveColor": LastMoveColor,
		"CheckColor":    CheckColor,
		"Dead":          dead,
	}

	fns := template.FuncMap{
		"GetID":            func(row, col int) int { return GetID(row, col, isFlipped) },
		"IsBestMove":       sqIsBestMove,
		"IsLastMove":       sqIsLastMove,
		"PieceInCheck":     pieceInCheck,
		"GetPieceFileName": getPieceFileName,
		"GetPid": func(sq chess.Square) string {
			if sq.Rank() == chess.Rank1 || sq.Rank() == chess.Rank2 || sq.Rank() == chess.Rank7 || sq.Rank() == chess.Rank8 {
				return "piece_" + sq.String()
			}
			return ""
		},
		"GetPid1": func(sq chess.Square) string { return g.piecesCache[sq] },

		"Square": func(id int) chess.Square { return chess.Square(id) },
		"PieceFromSq": func(sq chess.Square) chess.Piece {
			game := chess.NewGame()
			boardMap := game.Position().Board().SquareMap()
			return boardMap[sq]
		},
		"PieceFromSq1": func(sq chess.Square) chess.Piece {
			boardMap := game.Position().Board().SquareMap()
			return boardMap[sq]
		},
		"DeadPieceFromSq": func(sq chess.Square) chess.Piece { return deadBoardMap[sq] },
		"css":             func(s string) template.CSS { return template.CSS(s) },
		"cssUrl":          func(s string) template.URL { return template.URL(s) },
	}

	var buf bytes.Buffer
	if err := utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf, data); err != nil {
		logrus.Error(err)
	}
	return buf.String()
}

func renderBoardPng(isFlipped bool) image.Image {
	ctx := gg.NewContext(boardSize, boardSize)
	for i := 0; i < 64; i++ {
		sq := chess.Square(i)
		renderSquare(ctx, sq, isFlipped)
	}
	return ctx.Image()
}

func XyForSquare(isFlipped bool, sq chess.Square) (x, y int) {
	fileIndex := int(sq.File())
	rankIndex := 7 - int(sq.Rank())
	x = fileIndex * sqSize
	y = rankIndex * sqSize
	if isFlipped {
		x = boardSize - x - sqSize
		y = boardSize - y - sqSize
	}
	return
}

func colorForSquare(sq chess.Square) color.RGBA {
	sqSum := int(sq.File()) + int(sq.Rank())
	if sqSum%2 == 0 {
		return color.RGBA{R: 165, G: 117, B: 81, A: 255}
	}
	return color.RGBA{R: 235, G: 209, B: 166, A: 255}
}

func renderSquare(ctx *gg.Context, sq chess.Square, isFlipped bool) {
	x, y := XyForSquare(isFlipped, sq)
	// draw square
	ctx.Push()
	ctx.SetColor(colorForSquare(sq))
	ctx.DrawRectangle(float64(x), float64(y), sqSize, sqSize)
	ctx.Fill()
	ctx.Pop()

	// Draw file/rank
	ctx.Push()
	ctx.SetColor(color.RGBA{R: 0, G: 0, B: 0, A: 180})
	if (!isFlipped && sq.Rank() == chess.Rank1) || (isFlipped && sq.Rank() == chess.Rank8) {
		ctx.DrawString(sq.File().String(), float64(x+sqSize-7), float64(y+sqSize-1))
	}
	if (!isFlipped && sq.File() == chess.FileA) || (isFlipped && sq.File() == chess.FileH) {
		ctx.DrawString(sq.Rank().String(), float64(x+1), float64(y+11))
	}
	ctx.Pop()
}

func (g *ChessGame) renderBoardHTML(moveIdx int, isFlipped bool, imgB64 string, bestMove *chess.Move) string {
	position := g.Game.Position()
	if moveIdx != 0 && moveIdx < len(g.Game.Positions()) {
		position = g.Game.Positions()[moveIdx]
	}
	out := g.renderBoardHTML1(moveIdx, position, isFlipped, imgB64, bestMove)
	return out
}

func (g *ChessGame) renderBoardB64(isFlipped bool) string {
	var buf bytes.Buffer
	img := renderBoardPng(isFlipped)
	_ = png.Encode(&buf, img)
	imgB64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return imgB64
}

func (g *ChessGame) DrawPlayerCard(moveIdx int, key string, isBlack, isYourTurn, soundsEnabled, canUseChessAnalyze bool) string {
	return g.drawPlayerCard(moveIdx, key, isBlack, false, isYourTurn, soundsEnabled, canUseChessAnalyze)
}

func (g *ChessGame) drawPlayerCard(moveIdx int, key string, isBlack, isSpectator, isYourTurn, soundsEnabled, canUseChessAnalyze bool) string {
	htmlTmpl := `
<style>
#p1Status {
}
#p2Status {
}
#p1Status, #p2Status {
	width: 16px; height: 16px; border-radius: 8px;
	background-color: darkred;
	display: inline-block;
}
#white-advantage:before { content: "{{ .WhiteAdvantage }}"; }
#white-advantage .score:after { content: "{{ .WhiteScore }}"; }
#black-advantage:before { content: "{{ .BlackAdvantage }}"; }
#black-advantage .score:after { content: "{{ .BlackScore }}"; }
#outcome:after { content: "{{ .Outcome }}"; }
.score { font-size: 11px; }
</style>
<table style="width: 100%; height: 100%;">
	<tr>
		<td align="center">
			<table style="aspect-ratio: 1/1; height: 70%; max-width: 90%;">
				<tr>
					<td style="padding: 10px 0;" colspan="2">
						<table>
							<tr>
								<td style="padding-right: 10px;"><div id="p1Status"></div></td>
								<td>
									<span style="color: #eee; vertical-align: bottom;">
										<span {{ .White.UserStyle | attr }}>@{{ .White.Username }}</span> (white) VS
										<span {{ .Black.UserStyle | attr }}>@{{ .Black.Username }}</span> (black)
									</span>
								</td>
								<td style="padding-left: 10px;"><div id="p2Status"></div></td>
							</tr>
						</table>
					</td>
				</tr>
				<tr>
					<td>
						<span style="color: #eee; display: inline-block;">
							(<span id="white-advantage" style="color: #888;" title="white advantage"><span class="score"></span></span> |
							<span id="black-advantage" style="color: #888;" title="black advantage"><span class="score"></span></span>)
						</span>
					</td>
					<td align="right" style="vertical-align: middle;">
						<a href="/settings/chat" rel="noopener noreferrer" target="_blank">
							{{ if .SoundsEnabled }}
								<img src="/public/img/sounds-enabled.png" style="height: 20px;" alt="" title="Sounds enabled" />
 							{{ else }}
								<img src="/public/img/no-sound.png" style="height: 20px;" alt="" title="Sounds disabled" />
 							{{ end }}
						</a>
					</td>
				</tr>
				<tr>
					<td colspan="2">
						{{ if .GameOver }}
							<div style="position: relative;">
								<iframe src="/chess/{{ .Key }}/form" style="position: absolute; top: 0; left: 0; border: 0px solid red; z-index: 999; width: 100%; height: 100%;"></iframe>
								<div style="aspect-ratio: 1/1; height: 70%;">
									{{ .Table }}
								</div>
							</div>
						{{ else if or .IsSpectator }}
							{{ .Table }}
						{{ else }}
							<div style="position: relative;">
								<iframe src="/chess/{{ .Key }}/form" style="position: absolute; top: 0; left: 0; border: 0px solid red; z-index: 999; width: 100%; height: 100%;"></iframe>
								<div style="aspect-ratio: 1/1; height: 70%;">
									{{ .Table }}
									<div style="height: 33px;"></div>
								</div>
							</div>
						{{ end }}
					</td>
				</tr>
				{{ if .IsSpectator }}
					<tr><td style="padding: 10px 0;" colspan="2"><a href="?{{ if not .IsFlipped }}r=1{{ end }}" style="color: #eee;">Flip board</a></td></tr>
				{{ end }}
				<tr style="height: 100%;">
					<td colspan="2">
						<div style="color: #eee; display: inline-block;">Outcome: <span id="outcome"></span></div>
						{{ if and .GameOver .CanUseChessAnalyze }}
							<a style="color: #eee; margin-left: 20px;" href="/chess/{{ .Key }}/analyze">Analyze</a>
						{{ end }}
					</td>
				</tr>
				{{ if .GameOver }}<tr><td colspan="2"><div><textarea readonly>{{ .PGN }}</textarea></div></td></tr>{{ end }}
				{{ if .Stats }}
					<tr>
						<td colspan="2">
							<iframe name="iframeStats" src="/chess/{{ .Key }}/stats" style="width: 100%; height: 240px; margin: 10px 0; border: 3px solid black;"></iframe>
							{{ if .IsAnalyzed }}
								<div style="color: #eee;">White accuracy: <span id="white-accuracy">{{ .WhiteAccuracy | pct }}</span></div>
								<div style="color: #eee;">Black accuracy: <span id="black-accuracy">{{ .BlackAccuracy | pct }}</span></div>
							{{ end }}
						</td>
					</tr>
				{{ end }}
			</table>
		</td>
	</tr>
</table>
`

	player1 := g.Player1
	player2 := g.Player2
	game := g.Game
	enemy := utils.Ternary(isBlack, player1, player2)
	imgB64 := g.renderBoardB64(isBlack)
	whiteAdvantage, whiteScore, blackAdvantage, blackScore := CalcAdvantage(game.Position())

	const graphWidth = 800
	var columnWidth = 1
	var stats *AnalyzeResult
	_ = json.Unmarshal(g.DbChessGame.Stats, &stats)
	var bestMove *chess.Move
	if stats != nil {
		if len(stats.Scores) > 0 {
			if moveIdx > 0 && moveIdx < len(g.Game.Positions()) {
				position := g.Game.Positions()[moveIdx]
				bestMoveStr := stats.Scores[moveIdx-1].BestMove
				var err error
				bestMove, err = chess.UCINotation{}.Decode(position, bestMoveStr)
				if err != nil {
					logrus.Error(err)
				}
			}
			columnWidth = utils.MaxInt(graphWidth/len(stats.Scores), 1)
		}
	}

	data := map[string]any{
		"SoundsEnabled":      soundsEnabled,
		"Key":                key,
		"IsFlipped":          isBlack,
		"IsSpectator":        isSpectator,
		"White":              player1,
		"Black":              player2,
		"Username":           enemy.Username,
		"Table":              template.HTML(g.renderBoardHTML(moveIdx, isBlack, imgB64, bestMove)),
		"ImgB64":             imgB64,
		"Outcome":            game.Outcome().String(),
		"GameOver":           game.Outcome() != chess.NoOutcome,
		"PGN":                game.String(),
		"WhiteAdvantage":     whiteAdvantage,
		"WhiteScore":         whiteScore,
		"BlackAdvantage":     blackAdvantage,
		"BlackScore":         blackScore,
		"IsAnalyzed":         g.DbChessGame.AccuracyWhite != 0 && g.DbChessGame.AccuracyBlack != 0,
		"WhiteAccuracy":      g.DbChessGame.AccuracyWhite,
		"BlackAccuracy":      g.DbChessGame.AccuracyBlack,
		"Stats":              stats,
		"ColumnWidth":        columnWidth,
		"CanUseChessAnalyze": canUseChessAnalyze,
		"MoveIdx":            moveIdx,
	}

	fns := template.FuncMap{
		"attr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
		"pct": func(v float64) string {
			return fmt.Sprintf("%.1f%%", v)
		},
	}

	var buf1 bytes.Buffer
	if err := utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf1, data); err != nil {
		logrus.Error(err)
	}
	return buf1.String()
}

func (g *ChessGame) DrawSpectatorCard(moveIdx int, key string, isFlipped, soundsEnabled, canUseChessAnalyze bool) string {
	return g.drawPlayerCard(moveIdx, key, isFlipped, true, false, soundsEnabled, canUseChessAnalyze)
}

func (g *ChessGame) SetAnalyzing() bool {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	if g.analyzing {
		return false
	}
	g.analyzing = true
	return true
}

func (g *ChessGame) UnsetAnalyzing() {
	g.mtx.Lock()
	defer g.mtx.Unlock()
	g.analyzing = false
	g.analyzeProgrss = ChessAnalyzeProgress{}
}

func (g *ChessGame) IsAnalyzing() bool {
	g.mtx.RLock()
	defer g.mtx.RUnlock()
	return g.analyzing
}

func (b *Chess) GetGame(key string) (*ChessGame, error) {
	b.Lock()
	defer b.Unlock()
	dbChessGame, err := b.db.GetChessGame(key)
	if err != nil {
		return nil, err
	}
	if g, ok := b.games[key]; ok {
		return g, nil
	}
	player1, _ := b.db.GetUserByID(dbChessGame.WhiteUserID)
	player2, _ := b.db.GetUserByID(dbChessGame.BlackUserID)
	g := newChessGame(key, player1, player2, dbChessGame)
	b.games[key] = g
	return g, nil
}

func (b *Chess) GetGames() (out []ChessGame) {
	b.Lock()
	defer b.Unlock()
	for _, v := range b.games {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return
}

func (b *Chess) NewGame1(roomKey string, roomID database.RoomID, player1, player2 database.User, color string) (*ChessGame, error) {
	if player1.ID == player2.ID {
		return nil, errors.New("can't play yourself")
	}
	if color == "r" {
		color = utils.RandChoice([]string{"w", "b"})
	}
	if color == "b" {
		player1, player2 = player2, player1
	}

	key := uuid.New().String()
	g, err := b.NewGame(key, player1, player2)
	if err != nil {
		return nil, err
	}

	zeroUser := dutils.GetZeroUser(b.db)
	dutils.SendNewChessGameMessages(b.db, key, roomKey, roomID, zeroUser, player1, player2)
	return g, nil
}

func (b *Chess) NewGame(gameKey string, user1, user2 database.User) (*ChessGame, error) {
	dbChessGame, err := b.db.CreateChessGame(gameKey, user1.ID, user2.ID)
	if err != nil {
		return nil, err
	}
	g := newChessGame(gameKey, user1, user2, dbChessGame)
	b.Lock()
	b.games[gameKey] = g
	b.Unlock()
	return g, nil
}

func (b *Chess) SendMove(gameKey string, userID database.UserID, g *ChessGame, c echo.Context) error {
	player1 := g.Player1
	player2 := g.Player2
	game := g.Game

	if (game.Position().Turn() == chess.White && userID != player1.ID) ||
		(game.Position().Turn() == chess.Black && userID != player2.ID) {
		return errors.New("not your turn")
	}

	moveIdx, _ := strconv.Atoi(c.Request().PostFormValue("move_idx"))
	if moveIdx < len(g.Game.Moves())-1 {
		return errors.New("double submission")
	}

	piecesCache := g.piecesCache

	currentPlayer := player1
	opponentPlayer := player2
	if game.Position().Turn() == chess.Black {
		currentPlayer = player2
		opponentPlayer = player1
	}

	selectedSquares := make([]chess.Square, 0)
	for i := 0; i < 64; i++ {
		if utils.DoParseBool(c.Request().PostFormValue("sq_" + strconv.Itoa(i))) {
			selectedSquares = append(selectedSquares, chess.Square(i))
		}
	}

	if len(selectedSquares) != 2 {
		return errors.New("must select 2 squares")
	}

	promo := chess.Queen
	switch c.Request().PostFormValue("promotion") {
	case "queen":
		promo = chess.Queen
	case "rook":
		promo = chess.Rook
	case "knight":
		promo = chess.Knight
	case "bishop":
		promo = chess.Bishop
	}

	fst := selectedSquares[0]
	scd := selectedSquares[1]

	compareSquares := func(sq1, sq2, wanted1, wanted2 chess.Square) bool {
		return (sq1 == wanted1 && sq2 == wanted2) ||
			(sq1 == wanted2 && sq2 == wanted1)
	}

	// WKSq -> White King Square | WKSC -> White King Side Castle
	isWKSq := func(m *chess.Move) bool { return m.S1() == chess.E1 || m.S2() == chess.E1 }
	isBKSq := func(m *chess.Move) bool { return m.S1() == chess.E8 || m.S2() == chess.E8 }
	isWKSC := func(m *chess.Move) bool { return isWKSq(m) && m.HasTag(chess.KingSideCastle) }
	isBKSC := func(m *chess.Move) bool { return isBKSq(m) && m.HasTag(chess.KingSideCastle) }
	isWQSC := func(m *chess.Move) bool { return isWKSq(m) && m.HasTag(chess.QueenSideCastle) }
	isBQSC := func(m *chess.Move) bool { return isBKSq(m) && m.HasTag(chess.QueenSideCastle) }

	var moveStr string
	validMoves := game.Position().ValidMoves()
	var found bool
	var mov chess.Move
	for _, move := range validMoves {
		if (compareSquares(fst, scd, move.S1(), move.S2()) && (move.Promo() == chess.NoPieceType || move.Promo() == promo)) ||
			(isWKSC(move) && compareSquares(fst, scd, chess.E1, chess.H1)) ||
			(isBKSC(move) && compareSquares(fst, scd, chess.E8, chess.H8)) ||
			(isWQSC(move) && compareSquares(fst, scd, chess.E1, chess.A1)) ||
			(isBQSC(move) && compareSquares(fst, scd, chess.E8, chess.A8)) {
			moveStr = chess.AlgebraicNotation{}.Encode(game.Position(), move)
			found = true
			mov = *move
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid move %s %s", fst, scd)
	}

	//fmt.Println(moveStr)

	_ = game.MoveStr(moveStr)
	g.lastUpdated = time.Now()
	g.DbChessGame.PGN = game.String()
	g.DbChessGame.Outcome = game.Outcome().String()
	g.DbChessGame.DoSave(b.db)
	idStr1 := piecesCache[mov.S1()]
	idStr2 := piecesCache[mov.S2()]
	idStr3 := ""

	if mov.S1().Rank() == chess.Rank5 && mov.S2().Rank() == chess.Rank6 && mov.HasTag(chess.EnPassant) {
		idStr3 = piecesCache[chess.NewSquare(mov.S2().File(), chess.Rank5)]
	} else if mov.S1().Rank() == chess.Rank4 && mov.S2().Rank() == chess.Rank3 && mov.HasTag(chess.EnPassant) {
		idStr3 = piecesCache[chess.NewSquare(mov.S2().File(), chess.Rank4)]
	}

	updatePiecesCache(piecesCache, mov)

	var checkIDStr string
	if mov.HasTag(chess.Check) {
		checkIDStr = utils.Ternary(game.Position().Turn() == chess.White, WhiteKingID, BlackKingID)
	}

	chessMov := ChessMove{
		IDStr1:     idStr1,
		IDStr2:     idStr2,
		EnPassant:  idStr3,
		CheckIDStr: checkIDStr,
		Move:       mov,
	}
	ChessPubSub.Pub(gameKey, chessMov)

	// Notify (pm) the opponent that you made a move
	if opponentPlayer.NotifyChessMove {
		msg := fmt.Sprintf("@%s played %s", currentPlayer.Username, moveStr)
		msg, _ = dutils.ColorifyTaggedUsers(msg, b.db.GetUsersByUsername)
		chatMsg, _ := b.db.CreateMsg(msg, msg, "", config.GeneralRoomID, b.zeroID, &opponentPlayer.ID)
		go func() {
			time.Sleep(30 * time.Second)
			_ = chatMsg.Delete(b.db)
		}()
	}

	return nil
}

func (g *ChessGame) IsBlack(userID database.UserID) bool {
	return userID == g.Player2.ID
}

func (g *ChessGame) IsPlayer(userID database.UserID) bool {
	return g.Player1.ID == userID || g.Player2.ID == userID
}

func (g *ChessGame) MakeMoves(movesStr string, db *database.DkfDB) {
	moves := strings.Split(movesStr, " ")
	for _, move := range moves {
		g.MoveStr(move)
	}
	g.DbChessGame.PGN = g.Game.String()
	g.DbChessGame.Outcome = g.Game.Outcome().String()
	g.DbChessGame.DoSave(db)
}

func (g *ChessGame) MoveStr(m string) {
	game := g.Game
	piecesCache := g.piecesCache
	validMoves := game.Position().ValidMoves()
	var mov chess.Move
	for _, move := range validMoves {
		moveStr := chess.AlgebraicNotation{}.Encode(game.Position(), move)
		if moveStr == m {
			mov = *move
			break
		}
	}

	updatePiecesCache(piecesCache, mov)

	_ = game.MoveStr(m)
}

const (
	WhiteKingID          = "piece_e1"
	BlackKingID          = "piece_e8"
	WhiteKingSideRookID  = "piece_h1"
	BlackKingSideRookID  = "piece_h8"
	WhiteQueenSideRookID = "piece_a1"
	BlackQueenSideRookID = "piece_a8"
)

func InitPiecesCache(moves []*chess.Move) map[chess.Square]string {
	piecesCache := make(map[chess.Square]string)
	game := chess.NewGame()
	pos := game.Position()
	for i := 0; i < 64; i++ {
		sq := chess.Square(i)
		if pos.Board().Piece(sq) != chess.NoPiece {
			piecesCache[sq] = "piece_" + sq.String()
		}
	}
	for _, m := range moves {
		updatePiecesCache(piecesCache, *m)
	}
	return piecesCache
}

func updatePiecesCache(piecesCache map[chess.Square]string, mov chess.Move) {
	idStr1 := piecesCache[mov.S1()]
	delete(piecesCache, mov.S1())
	delete(piecesCache, mov.S2())
	piecesCache[mov.S2()] = idStr1
	if mov.S1().Rank() == chess.Rank6 && mov.S2().Rank() == chess.Rank7 && mov.HasTag(chess.EnPassant) {
		delete(piecesCache, chess.NewSquare(mov.S2().File(), chess.Rank6))
	} else if mov.S1().Rank() == chess.Rank5 && mov.S2().Rank() == chess.Rank4 && mov.HasTag(chess.EnPassant) {
		delete(piecesCache, chess.NewSquare(mov.S2().File(), chess.Rank5))
	}
	if mov.S1() == chess.E1 && mov.HasTag(chess.KingSideCastle) {
		delete(piecesCache, chess.H1)
		piecesCache[chess.F1] = WhiteKingSideRookID
	} else if mov.S1() == chess.E8 && mov.HasTag(chess.KingSideCastle) {
		delete(piecesCache, chess.H8)
		piecesCache[chess.F8] = BlackKingSideRookID
	} else if mov.S1() == chess.E1 && mov.HasTag(chess.QueenSideCastle) {
		delete(piecesCache, chess.A1)
		piecesCache[chess.D1] = WhiteQueenSideRookID
	} else if mov.S1() == chess.E8 && mov.HasTag(chess.QueenSideCastle) {
		delete(piecesCache, chess.A8)
		piecesCache[chess.D8] = BlackQueenSideRookID
	}
}

// Creates a map of pieces on the board and their count
func pieceMap(board *chess.Board) map[chess.Piece]int {
	m := board.SquareMap()
	out := make(map[chess.Piece]int)
	for _, piece := range m {
		out[piece]++
	}
	return out
}

/**
white chess king	♔	U+2654	&#9812;	&#x2654;
white chess queen	♕	U+2655	&#9813;	&#x2655;
white chess rook	♖	U+2656	&#9814;	&#x2656;
white chess bishop	♗	U+2657	&#9815;	&#x2657;
white chess knight	♘	U+2658	&#9816;	&#x2658;
white chess pawn	♙	U+2659	&#9817;	&#x2659;
black chess king	♚	U+265A	&#9818;	&#x265A;
black chess queen	♛	U+265B	&#9819;	&#x265B;
black chess rook	♜	U+265C	&#9820;	&#x265C;
black chess bishop	♝	U+265D	&#9821;	&#x265D;
black chess knight	♞	U+265E	&#9822;	&#x265E;
black chess pawn	♟︎	U+265F	&#9823;	&#x265F;
*/

// CalcAdvantage ...
func CalcAdvantage(position *chess.Position) (string, string, string, string) {
	m := pieceMap(position.Board())
	var whiteAdvantage, blackAdvantage string
	var whiteScore, blackScore int
	diff := m[chess.WhiteQueen] - m[chess.BlackQueen]
	whiteScore += diff * 9
	blackScore += -diff * 9
	whiteAdvantage += strings.Repeat("♛", utils.MaxInt(diff, 0))
	blackAdvantage += strings.Repeat("♕", utils.MaxInt(-diff, 0))
	diff = m[chess.WhiteRook] - m[chess.BlackRook]
	whiteScore += diff * 5
	blackScore += -diff * 5
	whiteAdvantage += strings.Repeat("♜", utils.MaxInt(diff, 0))
	blackAdvantage += strings.Repeat("♖", utils.MaxInt(-diff, 0))
	diff = m[chess.WhiteBishop] - m[chess.BlackBishop]
	whiteScore += diff * 3
	blackScore += -diff * 3
	whiteAdvantage += strings.Repeat("♝", utils.MaxInt(diff, 0))
	blackAdvantage += strings.Repeat("♗", utils.MaxInt(-diff, 0))
	diff = m[chess.WhiteKnight] - m[chess.BlackKnight]
	whiteScore += diff * 3
	blackScore += -diff * 3
	whiteAdvantage += strings.Repeat("♞", utils.MaxInt(diff, 0))
	blackAdvantage += strings.Repeat("♘", utils.MaxInt(-diff, 0))
	diff = m[chess.WhitePawn] - m[chess.BlackPawn]
	whiteScore += diff * 1
	blackScore += -diff * 1
	whiteAdvantage += strings.Repeat("♟", utils.MaxInt(diff, 0))
	blackAdvantage += strings.Repeat("♙", utils.MaxInt(-diff, 0))
	var whiteScoreLbl, blackScoreLbl string
	if whiteScore > 0 {
		whiteScoreLbl = fmt.Sprintf("+%d", whiteScore)
	}
	if blackScore > 0 {
		blackScoreLbl = fmt.Sprintf("+%d", blackScore)
	}
	if whiteAdvantage == "" {
		whiteAdvantage = "-"
	}
	if blackAdvantage == "" {
		blackAdvantage = "-"
	}
	return whiteAdvantage, whiteScoreLbl, blackAdvantage, blackScoreLbl
}

type Score struct {
	Move     string
	BestMove string
	CP       int
	Mate     int
}

type AnalyzeResult struct {
	WhiteAccuracy float64
	BlackAccuracy float64
	Scores        []Score
}

func AnalyzeGame(gg *ChessGame, pgn string, t int64) (out AnalyzeResult, err error) {
	pgnOpt, _ := chess.PGN(strings.NewReader(pgn))
	g := chess.NewGame(pgnOpt)
	positions := g.Positions()
	nbPosition := len(positions)

	pubProgress := func(step int) {
		progress := ChessAnalyzeProgress{Step: step, Total: nbPosition}
		gg.SetAnalyzeProgress(progress)
		pubKey := "chess_analyze_progress_" + gg.Key
		ChessAnalyzeProgressPubSub.Pub(pubKey, progress)
	}
	defer func() {
		pubProgress(nbPosition)
		gg.UnsetAnalyzing()
	}()

	eng, err := uci.New("stockfish")
	if err != nil {
		logrus.Error(err)
		return out, err
	}
	if err := eng.Run(uci.CmdUCI, uci.CmdIsReady, uci.CmdUCINewGame); err != nil {
		logrus.Error(err)
		return out, err
	}
	defer eng.Close()

	scores := make([]Score, 0)
	cps := make([]int, 0)

	t = utils.Clamp(t, 15, 60)
	moveTime := time.Duration((float64(t)/float64(len(positions)-1))*1000) * time.Millisecond

	for idx, position := range positions {
		// First position is the board without any move played
		if idx == 0 {
			continue
		}
		cmdPos := uci.CmdPosition{Position: position}
		cmdGo := uci.CmdGo{MoveTime: moveTime}
		if err := eng.Run(cmdPos, cmdGo); err != nil {
			logrus.Error(err)
			mov := g.MoveHistory()[idx-1].Move
			moveStr := chess.AlgebraicNotation{}.Encode(positions[idx-1], mov)
			cps = append(cps, 0)
			scores = append(scores, Score{Move: moveStr})
			pubProgress(idx)
			continue
		}
		res := eng.SearchResults()
		cp := res.Info.Score.CP
		mate := res.Info.Score.Mate
		if idx%2 != 0 {
			cp *= -1
			mate *= -1
		}
		mov := g.MoveHistory()[idx-1].Move
		moveStr := chess.AlgebraicNotation{}.Encode(positions[idx-1], mov)
		bestMoveStr := chess.UCINotation{}.Encode(position, res.BestMove)
		cps = append(cps, cp)
		scores = append(scores, Score{Move: moveStr, BestMove: bestMoveStr, CP: cp, Mate: mate})

		pubProgress(idx)
	}

	//fmt.Println(strings.Join(s, ", "))

	wa, ba := gameAccuracy(cps)
	return AnalyzeResult{
		Scores:        scores,
		WhiteAccuracy: wa,
		BlackAccuracy: ba,
	}, nil
}

func mean(arr []float64) float64 {
	var sum float64
	for _, n := range arr {
		sum += n
	}
	return sum / float64(len(arr))
}

func standardDeviation(arr []float64) float64 {
	nb := float64(len(arr))
	m := mean(arr)
	var acc float64
	for _, n := range arr {
		acc += (n - m) * (n - m)
	}
	return math.Sqrt(acc / nb)
}

type Cp int

const CpCeiling = Cp(1000)
const CpInitial = Cp(15)

func (c Cp) ceiled() Cp {
	if c > CpCeiling {
		return CpCeiling
	} else if c < -CpCeiling {
		return -CpCeiling
	}
	return c
}

func fromCentiPawns(cp Cp) float64 {
	return 50 + 50*winningChances(cp.ceiled())
}

func winningChances(cp Cp) float64 {
	const MULTIPLIER = -0.00368208 // https://github.com/lichess-org/lila/pull/11148
	res := 2/(1+math.Exp(MULTIPLIER*float64(cp))) - 1
	out := math.Max(math.Min(res, 1), -1)
	return out
}

func fromWinPercents(before, after float64) (accuracy float64) {
	if after >= before {
		return 100
	}
	winDiff := before - after
	raw := 103.1668100711649*math.Exp(-0.04354415386753951*winDiff) + -3.166924740191411
	raw += 1
	return math.Min(math.Max(raw, 0), 100)
}

func calcWindows(allWinPercents []float64, windowSize int) (out [][]float64) {
	start := allWinPercents[:windowSize]
	m := utils.MinInt(windowSize, len(allWinPercents))
	for i := 0; i < m-2; i++ {
		out = append(out, start)
	}

	for i := 0; i < len(allWinPercents)-(windowSize-1); i++ {
		curr := make([]float64, 0)
		for j := 0; j < windowSize; j++ {
			curr = append(curr, allWinPercents[i+j])
		}
		out = append(out, curr)
	}
	return
}

func calcWeights(windows [][]float64) (out []float64) {
	out = make([]float64, len(windows))
	for i, w := range windows {
		out[i] = math.Min(math.Max(standardDeviation(w), 0.5), 12)
	}
	return
}

func calcWeightedAccuracies(allWinPercents []float64, weights []float64) (float64, float64) {
	sw := calcWindows(allWinPercents, 2)
	whites := make([][2]float64, 0)
	blacks := make([][2]float64, 0)
	for i := 0; i < len(sw); i++ {
		prev, next := sw[i][0], sw[i][1]
		acc := prev
		acc1 := next
		if i%2 != 0 {
			acc, acc1 = acc1, acc
		}
		accuracy := fromWinPercents(float64(acc), float64(acc1))
		el := [2]float64{accuracy, weights[i]}
		if i%2 == 0 {
			whites = append(whites, el)
		} else {
			blacks = append(blacks, el)
		}
	}

	www1 := weightedMean(whites)
	www2 := harmonicMean(whites)
	bbb1 := weightedMean(blacks)
	bbb2 := harmonicMean(blacks)
	return (www1 + www2) / 2, (bbb1 + bbb2) / 2
}

func harmonicMean(arr [][2]float64) float64 {
	vs := make([]float64, 0)
	for _, v := range arr {
		vs = append(vs, v[0])
	}
	var sm float64
	for _, v := range vs {
		sm += 1 / math.Max(1, v)
	}
	return float64(len(vs)) / sm
}

/**
  def harmonicMean(a: Iterable[Double]): Option[Double] =
    a.nonEmpty option {
      a.size / a.foldLeft(0d) { (acc, v) => acc + 1 / Math.max(1, v) }
    }

  def weightedMean(a: Iterable[(Double, Double)]): Option[Double] =
    a.nonEmpty so {
      a.foldLeft(0d -> 0d) { case ((av, aw), (v, w)) => (av + v * w, aw + w) } match
        case (v, w) => w != 0 option v / w
    }
*/

func weightedMean(a [][2]float64) float64 {
	vs := make([]float64, 0)
	ws := make([]float64, 0)

	for _, v := range a {
		vs = append(vs, v[0])
		ws = append(ws, v[1])
	}

	sumWeight, avg := 0.0, 0.0
	for i, v := range vs {
		if v == 0 {
			continue
		}
		sumWeight += ws[i]
		avg += v * ws[i]
	}
	avg /= sumWeight
	return avg
}

func gameAccuracy(cps []int) (float64, float64) {
	cps = append([]int{int(CpInitial)}, cps...)
	var allWinPercents []float64
	for _, cp := range cps {
		allWinPercents = append(allWinPercents, fromCentiPawns(Cp(cp)))
	}
	windowSize := int(math.Min(math.Max(float64(len(cps)/10), 2), 8))
	windows := calcWindows(allWinPercents, windowSize)
	weights := calcWeights(windows)
	wa, ba := calcWeightedAccuracies(allWinPercents, weights)
	return wa, ba
}
