package v1

import (
	"bytes"
	"dkforest/bindata"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/fogleman/gg"
	"github.com/google/uuid"
	"github.com/labstack/echo"
	"github.com/notnil/chess"
	"github.com/sirupsen/logrus"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"sort"
	"strconv"
	"sync"
	"time"
)

type ChessPlayer struct {
	ID              database.UserID
	Username        string
	UserStyle       string
	NotifyChessMove bool
}

type ChessGame struct {
	Key         string
	Game        *chess.Game
	lastUpdated time.Time
	Player1     *ChessPlayer
	Player2     *ChessPlayer
	CreatedAt   time.Time
}

func newChessPlayer(player database.User) *ChessPlayer {
	p := new(ChessPlayer)
	p.ID = player.ID
	p.Username = player.Username
	p.UserStyle = player.GenerateChatStyle()
	p.NotifyChessMove = player.NotifyChessMove
	return p
}

func newChessGame(gameKey string, player1, player2 database.User) *ChessGame {
	g := new(ChessGame)
	g.CreatedAt = time.Now()
	g.Key = gameKey
	g.Game = chess.NewGame()
	g.lastUpdated = time.Now()
	g.Player1 = newChessPlayer(player1)
	g.Player2 = newChessPlayer(player2)
	return g
}

type Chess struct {
	sync.Mutex
	zeroID database.UserID
	games  map[string]*ChessGame
}

func NewChess() *Chess {
	zeroUser, _ := database.GetUserByUsername(config.NullUsername)
	c := &Chess{zeroID: zeroUser.ID}
	c.games = make(map[string]*ChessGame)

	// Thread that cleanup inactive games
	go func() {
		for {
			time.Sleep(time.Minute)
			c.Lock()
			for k, g := range c.games {
				if time.Since(g.lastUpdated) > 30*time.Minute {
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
)

func renderBoardPng(last *chess.Move, position *chess.Position, isFlipped bool) image.Image {
	boardMap := position.Board().SquareMap()
	ctx := gg.NewContext(boardSize, boardSize)
	for i := 0; i < 64; i++ {
		sq := chess.Square(i)
		sqPiece := boardMap[sq]
		renderSquare(ctx, sq, last, position.Turn(), sqPiece, isFlipped)
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

func renderSquare(ctx *gg.Context, sq chess.Square, last *chess.Move, turn chess.Color, sqPiece chess.Piece, isFlipped bool) {
	x, y := XyForSquare(isFlipped, sq)
	// draw square
	ctx.Push()
	ctx.SetColor(colorForSquare(sq))
	ctx.DrawRectangle(float64(x), float64(y), sqSize, sqSize)
	ctx.Fill()
	ctx.Pop()
	// Draw previous move
	if last != nil {
		if last.S1() == sq ||
			last.S2() == sq {
			ctx.Push()
			ctx.SetRGBA(0, 1, 0, 0.1)
			ctx.DrawRectangle(float64(x), float64(y), sqSize, sqSize)
			ctx.Fill()
			ctx.Pop()
		}
		// Draw check
		p := sqPiece
		if p != chess.NoPiece {
			if p.Type() == chess.King && p.Color() == turn && last.HasTag(chess.Check) {
				ctx.Push()
				ctx.SetRGBA(1, 0, 0, 0.4)
				ctx.DrawRectangle(float64(x), float64(y), sqSize, sqSize)
				ctx.Fill()
				ctx.Pop()
			}
		}
	}

	// draw piece
	p := sqPiece
	if p != chess.NoPiece {
		img := getFile("img/chess/" + p.Color().String() + pieceTypeMap[p.Type()] + ".png")

		ctx.Push()
		ctx.DrawImage(img, x, y)
		ctx.Pop()
	}
}

var pieceTypeMap = map[chess.PieceType]string{
	chess.King:   "K",
	chess.Queen:  "Q",
	chess.Rook:   "R",
	chess.Bishop: "B",
	chess.Knight: "N",
	chess.Pawn:   "P",
}

var cache = make(map[string]image.Image)

func getFile(fileName string) image.Image {
	if img, ok := cache[fileName]; ok {
		return img
	}
	fileBy := bindata.MustAsset(fileName)
	img, _ := png.Decode(bytes.NewReader(fileBy))
	cache[fileName] = img
	return img
}

func renderTable(imgB64 string, isBlack bool) string {
	htmlTmpl := `
<style>
input[type=checkbox] {
    display:none;
}
input[type=checkbox] + label {
    display: inline-block;
    padding: 0 0 0 0;
	margin: 0 0 0 0;
    height: 39px;
    width: 39px;
    background-size: 100%;
	border: 3px solid transparent;
}
input[type=checkbox]:checked + label {
    display: inline-block;
    background-size: 100%;
	border: 3px solid red;
}
</style>

<table style="width: 360px; height: 360px; background-image: url(data:image/png;base64,{{ .ImgB64 }})">
	{{ range $row := .Rows }}
		<tr>
			{{ range $col := $.Cols }}
				{{ $id := GetID $row $col }}
				<td>
					<input name="sq_{{ $id }}" ID="sq_{{ $id }}" type="checkbox" value="1" />
					<label for="sq_{{ $id }}"></label>
				</td>
			{{ end }}
		</tr>
	{{ end }}
</table>
`
	data := map[string]any{
		"ImgB64": imgB64,
		"Rows":   []int{0, 1, 2, 3, 4, 5, 6, 7},
		"Cols":   []int{0, 1, 2, 3, 4, 5, 6, 7},
	}

	fns := template.FuncMap{
		"GetID": func(row, col int) int {
			var id int
			if isBlack {
				id = row*8 + (7 - col)
			} else {
				id = (7-row)*8 + col
			}
			return id
		},
	}

	var buf bytes.Buffer
	_ = utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (g *ChessGame) renderBoardB64(isFlipped bool) string {
	position := g.Game.Position()
	var last *chess.Move
	if len(g.Game.Moves()) > 0 {
		last = g.Game.Moves()[len(g.Game.Moves())-1]
	}
	var buf bytes.Buffer
	img := renderBoardPng(last, position, isFlipped)
	_ = png.Encode(&buf, img)
	imgB64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return imgB64
}

func (g *ChessGame) DrawPlayerCard(inChat, isBlack, isYourTurn bool) string {
	enemy := g.Player2
	if isBlack {
		enemy = g.Player1
	}

	imgB64 := g.renderBoardB64(isBlack)

	htmlTmpl := `
<div style="color: #eee;">
	<span {{ .White.UserStyle | attr }}>@{{ .White.Username }}</span> (white) VS
	<span {{ .Black.UserStyle | attr }}>@{{ .Black.Username }}</span> (black)
</div>

{{ if .GameOver }}
	<div style="width: 360px; height: 360px; background-image: url(data:image/png;base64,{{ .ImgB64 }})"></div>
{{ else }}
	<form method="post">
		<input type="hidden" name="message" value="resign" />
		<button type="submit">Resign</button>
	</form>
	{{ if .IsYourTurn }}
		<form method="post"{{ if .InChat }} action="/api/v1/chat/top-bar/chess" target="iframe1"{{ end }}>
			{{ .Table }}
			<input type="hidden" name="message" value="/pm {{ .Username }} /c move" />
			<button type="submit">Move</button>
		</form>
	{{ else }}
		<div style="width: 360px; height: 360px; background-image: url(data:image/png;base64,{{ .ImgB64 }})"></div>
	{{ end }}
{{ end }}
<div style="color: #eee;">Outcome: {{ .Outcome }}</div>

{{ if .GameOver }}
	<div><textarea>{{ .PGN }}</textarea></div>
{{ end }}
`

	data := map[string]any{
		"IsYourTurn": isYourTurn,
		"InChat":     inChat,
		"White":      g.Player1,
		"Black":      g.Player2,
		"Username":   enemy.Username,
		"Table":      template.HTML(renderTable(imgB64, isBlack)),
		"ImgB64":     imgB64,
		"Outcome":    g.Game.Outcome().String(),
		"GameOver":   g.Game.Outcome() != chess.NoOutcome,
		"PGN":        g.Game.String(),
	}

	fns := template.FuncMap{
		"attr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
	}

	var buf1 bytes.Buffer
	_ = utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf1, data)
	return buf1.String()
}

func (g *ChessGame) DrawSpectatorCard(isFlipped bool) string {
	imgB64 := g.renderBoardB64(isFlipped)

	htmlTmpl := `
<div style="color: #eee;">
	<span {{ .White.UserStyle | attr }}>@{{ .White.Username }}</span> (white) VS
	<span {{ .Black.UserStyle | attr }}>@{{ .Black.Username }}</span> (black)
</div>
<div style="width: 360px; height: 360px; background-image: url(data:image/png;base64,{{ .ImgB64 }})"></div>
<div style="color: #eee;">Outcome: {{ .Outcome }}</div>
{{ if .GameOver }}
	<div><textarea>{{ .PGN }}</textarea></div>
{{ end }}
`

	data := map[string]any{
		"White":    g.Player1,
		"Black":    g.Player2,
		"ImgB64":   imgB64,
		"Outcome":  g.Game.Outcome().String(),
		"GameOver": g.Game.Outcome() != chess.NoOutcome,
		"PGN":      g.Game.String(),
	}

	fns := template.FuncMap{
		"attr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
	}

	var buf1 bytes.Buffer
	_ = utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf1, data)
	return buf1.String()
}

func (b *Chess) GetGame(key string) *ChessGame {
	b.Lock()
	defer b.Unlock()
	if g, ok := b.games[key]; ok {
		return g
	}
	return nil
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

func (b *Chess) NewGame1(roomKey string, roomID database.RoomID, player1, player2 database.User) (*ChessGame, error) {
	if player1.ID == player2.ID {
		return nil, errors.New("can't play yourself")
	}

	key := uuid.New().String()
	g := b.NewGame(key, player1, player2)

	zeroUser := dutils.GetZeroUser()
	dutils.SendNewChessGameMessages(key, roomKey, roomID, zeroUser, player1, player2)
	return g, nil
}

func (b *Chess) NewGame(gameKey string, user1, user2 database.User) *ChessGame {
	g := newChessGame(gameKey, user1, user2)
	b.Lock()
	b.games[gameKey] = g
	b.Unlock()
	return g
}

func (b *Chess) SendMove(gameKey string, userID database.UserID, g *ChessGame, c echo.Context) error {
	if (g.Game.Position().Turn() == chess.White && userID != g.Player1.ID) ||
		(g.Game.Position().Turn() == chess.Black && userID != g.Player2.ID) {
		return errors.New("not your turn")
	}

	you := g.Player2
	opponent := g.Player1
	if g.Game.Position().Turn() == chess.White {
		you = g.Player1
		opponent = g.Player2
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

	var moveStr string
	validMoves := g.Game.Position().ValidMoves()
	var found bool
	for _, move := range validMoves {
		if (move.S1() == selectedSquares[0] && move.S2() == selectedSquares[1]) ||
			(move.S1() == selectedSquares[1] && move.S2() == selectedSquares[0]) {
			moveStr = chess.AlgebraicNotation{}.Encode(g.Game.Position(), move)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid move %s %s", selectedSquares[0], selectedSquares[1])
	}

	_ = g.Game.MoveStr(moveStr)
	g.lastUpdated = time.Now()
	if g.Game.Outcome() != chess.NoOutcome {
		//delete(b.games, gameKey)
	}

	pubsub.Pub(gameKey, true)

	// Notify (pm) the opponent that you made a move
	if opponent.NotifyChessMove {
		msg := fmt.Sprintf("@%s played %s", you.Username, moveStr)
		msg, _ = colorifyTaggedUsers(msg, database.GetUsersByUsername)
		chatMsg, _ := database.CreateMsg(msg, msg, "", config.GeneralRoomID, b.zeroID, &opponent.ID)
		go func() {
			time.Sleep(30 * time.Second)
			_ = database.DeleteChatMessageByUUID(chatMsg.UUID)
		}()
	}

	return nil
}

func (b *Chess) InterceptMsg(cmd *Command) {
	b.Lock()
	defer b.Unlock()

	m := cRgx.FindStringSubmatch(cmd.message)
	if len(m) != 3 {
		return
	}

	user, err := database.GetUserByUsername(m[1])
	if err != nil {
		cmd.err = errors.New("invalid username")
		return
	}

	var gameKey string
	if cmd.fromUserID < user.ID {
		gameKey = fmt.Sprintf("%d_%d", cmd.fromUserID, user.ID)
	} else {
		gameKey = fmt.Sprintf("%d_%d", user.ID, cmd.fromUserID)
	}

	pos := m[2]

	g, ok := b.games[gameKey]
	if ok {
		if err := b.SendMove(gameKey, cmd.fromUserID, g, cmd.c); err != nil {
			cmd.err = err
			return
		}
	} else {
		if pos != "" {
			cmd.err = errors.New("no Game ongoing")
			return
		}
		g = b.NewGame(gameKey, user, *cmd.authUser)
	}

	// Delete old messages sent by "0" to the players
	if err := database.DB.
		Where("room_id = ? AND user_id = ? AND (to_user_id = ? OR to_user_id = ?)", cmd.room.ID, b.zeroID, g.Player1.ID, g.Player2.ID).
		Delete(&database.ChatMessage{}).Error; err != nil {
		logrus.Error(err)
	}

	card1 := g.DrawPlayerCard(true, false, true)
	_, _ = database.CreateMsg(card1, card1, cmd.roomKey, cmd.room.ID, b.zeroID, &g.Player1.ID)

	card1 = g.DrawPlayerCard(true, true, true)
	_, _ = database.CreateMsg(card1, card1, cmd.roomKey, cmd.room.ID, b.zeroID, &g.Player2.ID)

	cmd.err = ErrStop
}
