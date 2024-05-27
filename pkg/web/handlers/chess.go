package handlers

import (
	"bytes"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/hashset"
	"dkforest/pkg/pubsub"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors"
	"dkforest/pkg/web/handlers/usersStreamsManager"
	hutils "dkforest/pkg/web/handlers/utils"
	"dkforest/pkg/web/handlers/utils/stream"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/labstack/echo"
	"github.com/notnil/chess"
	"github.com/sirupsen/logrus"
	"html/template"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type StylesBuilder []string

func (b *StylesBuilder) Append(v string) {
	*b = append(*b, v)
}

func (b *StylesBuilder) Appendf(format string, a ...any) {
	b.Append(fmt.Sprintf(format, a...))
}

func (b *StylesBuilder) Build() string {
	return fmt.Sprintf("<style>%s</style>", strings.Join(*b, " "))
}

const (
	foolMateGame        = "f3 e5 g4"
	checkGame           = "Nc3 h6 Nb5 h5"
	promoWGame          = "h4 g5 hxg5 h5 g6 h4 g7 h3"
	promoBGame          = "a3 c5 a4 c4 a5 c3 a6 cxb2 axb7"
	kingSideCastleGame  = "e3 e6 Be2 Be7 Nf3 Nf6"
	queenSideCastleGame = "d4 d5 Qd3 Qd6 Bd2 Bd7 Nc3 Nc6"
	enPassantGame       = "d4 f6 d5 e5"
	staleMateGame       = "d4 d5 Nf3 Nf6 Bf4 Bg4 e3 e6 Bd3 c6 c3 Bd6 Bg3 Bxg3 hxg3 Nbd7 Nbd2 Ne4 Bxe4 dxe4 Nxe4 f5 Ned2 Qf6 Qa4 Nb6 Qb4 Qe7 Qxe7+ Kxe7 Ne5 Nd7 f3 Bh5 Rxh5 Nxe5 dxe5 g6 Rd1 Rad8 Nc4 Rxd1+ Kxd1 gxh5 Nd6 Rg8 Nxb7 Rxg3 Nc5 Rxg2 Kc1 Re2 e4 fxe4 Nxe4 h4 Ng5 h6 Nh3 Rh2 Nf4 h3 a4 Rh1+ Kc2 h2 Nh3 Rf1 f4 h1=Q f5 Qxh3 Kb3 Qxf5 a5 Qxe5 a6 Ra1 c4 Qe3+ Kb4 h5 b3 h4 c5 h3 Kc4 h2 Kb4"
)

func ChessHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	var data chessData
	data.Games = interceptors.ChessInstance.GetGames()

	if c.Request().Method == http.MethodPost {
		data.Username = database.Username(c.Request().PostFormValue("username"))
		data.Color = c.Request().PostFormValue("color")
		player1 := *authUser
		player2, err := db.GetUserByUsername(data.Username)
		if err != nil {
			data.Error = "invalid username"
			return c.Render(http.StatusOK, "chess", data)
		}
		if _, err := interceptors.ChessInstance.NewGame1("", config.GeneralRoomID, player1, player2, data.Color); err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "chess", data)
		}
		return hutils.RedirectReferer(c)
	}

	return c.Render(http.StatusOK, "chess", data)
}

func ChessGameAnalyzeHandler(c echo.Context) error {
	key := c.Param("key")
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)
	csrf, _ := c.Get("csrf").(string)
	if !authUser.CanUseChessAnalyze {
		return c.Redirect(http.StatusFound, "/")
	}
	g, err := interceptors.ChessInstance.GetGame(key)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	game := g.Game
	if game.Outcome() == chess.NoOutcome {
		return c.String(http.StatusOK, "no outcome")
	}

	if c.Request().Method == http.MethodGet && !g.IsAnalyzing() {
		return c.HTML(http.StatusOK, `
<style>html, body { background-color: #222; color: #eee; }</style>
<form method="post">
	<input type="hidden" name="csrf" value="`+csrf+`" />
	Total time (15-60):
	<input type="number" name="t" value="15" min="15" max=60 />
	<button type="submit">Start analyze</button>
</form>`)
	}

	t := utils.Clamp(utils.ParseInt64OrDefault(c.Request().PostFormValue("t"), 15), 15, 60)
	db.NewAudit(*authUser, fmt.Sprintf("start chess analyze: t=%d | key=%s", t, g.Key))

	if g.SetAnalyzing() {
		go func() {
			res, err := interceptors.AnalyzeGame(g, game.String(), t)
			if err != nil {
				logrus.Error(err)
				return
			}
			g.DbChessGame.Stats, _ = json.Marshal(res)
			g.DbChessGame.AccuracyWhite = res.WhiteAccuracy
			g.DbChessGame.AccuracyBlack = res.BlackAccuracy
			g.DbChessGame.DoSave(db)
		}()
	}

	streamItem, err := stream.SetStreaming(c, authUser.ID, "analyze_"+key)
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	sub := interceptors.ChessAnalyzeProgressPubSub.Subscribe([]string{"chess_analyze_progress_" + key})
	defer sub.Close()

	renderProgress := func(progress interceptors.ChessAnalyzeProgress) {
		_, _ = c.Response().Write([]byte(fmt.Sprintf(`<style>#progress:after { content: "PROGRESS: %d/%d" }</style>`, progress.Step, progress.Total)))
		c.Response().Flush()
	}

	_, _ = c.Response().Write([]byte(`<style>html, body { background-color: #222; }
#progress { color: #eee; }
</style>`))
	_, _ = c.Response().Write([]byte(`<div id="progress"></div>`))
	progress := g.GetAnalyzeProgress()
	renderProgress(progress)

	defer func() {
		_, _ = c.Response().Write([]byte(fmt.Sprintf(`<a href="/chess/%s">Back</a>`, g.Key)))
		c.Response().Flush()
	}()

Loop:
	for {
		select {
		case <-streamItem.Quit:
			break Loop
		default:
		}

		if progress.Step > 0 && progress.Step == progress.Total {
			break
		}

		_, progress, err = sub.ReceiveTimeout2(1*time.Second, streamItem.Quit)
		if err != nil {
			if errors.Is(err, pubsub.ErrCancelled) {
				break Loop
			}
			continue
		}

		renderProgress(progress)
	}

	return nil
}

func ChessGameStatsHandler(c echo.Context) error {
	key := c.Param("key")
	authUser := c.Get("authUser").(*database.User)
	csrf, _ := c.Get("csrf").(string)
	g, err := interceptors.ChessInstance.GetGame(key)
	if err != nil {
		return c.NoContent(http.StatusOK)
	}
	htmlTmpl := hutils.HtmlCssReset + `
<style>
.graph {
	border: 0px solid #000;
	background-color: #666;
	box-sizing: border-box;
	width: 100%;
	table-layout: fixed;
}
.graph tr { height: 240px; }
.graph td {
	height: inherit;
	border-right: 0px solid #555;
}
.graph td:hover {
	background-color: #5c5c5c;
}
.graph form {
	height: 100%;
	position: relative;
	border: none;
}
.graph .column-wrapper-wrapper {
	height: 100%;
	width: 100%;
	position: relative;
	border: none;
	background-color: transparent;
	cursor: pointer;
	padding: 0;
}
.graph .column-wrapper {
	height: 50%;
	width: 100%;
	position: relative;
}
.graph .column {
	position: absolute;
	width: 100%;
	box-sizing: border-box;
	border-right: 1px solid #555;
}
</style>
<form method="post">
	<input type="hidden" name="csrf" value="{{ $.CSRF }}" />
	<table class="graph">
		<tr>
			{{ range $idx, $el := .Stats.Scores }}
				<td title="{{ $idx | fmtMove }} {{ $el.Move }} | Advantage: {{ if not $el.Mate }}{{ $el.CP | cp }}{{ else }}#{{ $el.Mate }}{{ end }}">
					{{ $el.BestMove | commentHTML }}
					<button type="submit" name="move_idx" value="{{ $idx | plus }}" class="column-wrapper-wrapper" style="display: block;{{ if eq $.MoveIdx ($idx | plus) }} background-color: rgba(255, 255, 0, 0.2);{{ end }}">
						<div class="column-wrapper" style="border-bottom: 1px solid #333; box-sizing: border-box;">
							{{ if ge .CP 0 }}
								<div class="column" style="height: {{ $el | renderCP "white" }}px; background-color: #eee; bottom: 0;"></div>
							{{ end }}
						</div>
						<div class="column-wrapper">
							{{ if le .CP 0 }}
								<div class="column" style="height: {{ $el | renderCP "black" }}px; background-color: #111;"></div>
							{{ end }}
						</div>
					</button>
				</td>
			{{ end }}
		</tr>
	</table>
</form>`

	data := map[string]any{
		"CSRF": csrf,
	}

	fns := template.FuncMap{
		"commentHTML": func(s string) template.HTML {
			return template.HTML(fmt.Sprintf("<!-- %s -->", s))
		},
		"attr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
		"plus": func(v int) int { return v + 1 },
		"pct": func(v float64) string {
			return fmt.Sprintf("%.1f%%", v)
		},
		"abs": func(v int) int { return int(math.Abs(float64(v))) },
		"cp": func(v int) string {
			return fmt.Sprintf("%.2f", float64(v)/100)
		},
		"fmtMove": func(idx int) string {
			idx += 2
			if idx%2 == 0 {
				return fmt.Sprintf("%d.", idx/2)
			}
			return fmt.Sprintf("%d...", idx/2)
		},
		"renderCP": func(color string, v interceptors.Score) int {
			const maxH = 120  // Max graph height
			const maxV = 1200 // Max cp value. Anything bigger should take 100% of graph height space.
			absV := int(math.Abs(float64(v.CP)))
			absV = utils.MinInt(absV, maxV)
			absV = absV * maxH / maxV
			if v.CP == 0 && v.Mate != 0 {
				if (color == "white" && v.Mate > 0) || (color == "black" && v.Mate < 0) {
					absV = maxH
				}
			}
			return absV
		},
	}

	currMoveIdx := -1
	v, err := c.Cookie("chess_" + key)
	if err == nil {
		currMoveIdx, _ = strconv.Atoi(v.Value)
	}

	moveIdx := currMoveIdx

	var stats *interceptors.AnalyzeResult
	if err := json.Unmarshal(g.DbChessGame.Stats, &stats); err != nil {
		return hutils.RedirectReferer(c)
	}

	if c.Request().Method == http.MethodPost {
		moveIdxStr := c.Request().PostFormValue("move_idx")
		if moveIdxStr == "" {
			moveIdx = -1
		}
		moveIdx, err = strconv.Atoi(moveIdxStr)
		if err != nil {
			moveIdx = -1
		}
		if moveIdx == -1 {
			moveIdx = currMoveIdx
		}
		btnSubmit := c.Request().PostFormValue("btn_submit")
		if moveIdx == -1 {
			moveIdx = len(stats.Scores)
		}
		if btnSubmit == "prev_position" {
			moveIdx -= 1
		} else if btnSubmit == "next_position" {
			moveIdx += 1
		}
	}

	moveIdx = utils.Clamp(moveIdx, 0, len(stats.Scores))
	c.SetCookie(hutils.CreateCookie("chess_"+key, strconv.Itoa(moveIdx), utils.OneDaySecs))
	var bestMove string
	if stats != nil {
		if len(stats.Scores) > 0 {
			if moveIdx > 0 {
				bestMove = stats.Scores[moveIdx-1].BestMove
			}
		}
	}
	interceptors.ChessPubSub.Pub(key+"_"+authUser.Username.String(), interceptors.ChessMove{MoveIdx: moveIdx, BestMove: bestMove})

	data["Stats"] = stats
	data["MoveIdx"] = moveIdx

	var buf1 bytes.Buffer
	if err := utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf1, data); err != nil {
		logrus.Error(err)
	}
	return c.HTML(http.StatusOK, buf1.String())
}

func ChessGameFormHandler(c echo.Context) error {
	key := c.Param("key")
	csrf, _ := c.Get("csrf").(string)
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)
	g, err := interceptors.ChessInstance.GetGame(key)
	if err != nil {
		return c.NoContent(http.StatusOK)
	}
	game := g.Game
	isFlipped := g.IsBlack(authUser.ID)

	if game.Outcome() != chess.NoOutcome {
		if g.DbChessGame.Stats == nil {
			return c.NoContent(http.StatusOK)
		}
		htmlTmpl := `
<style>
button {
	background-color: transparent;
	position: absolute;
	top: 0;
	bottom: 0;
	border: none;
}
#prev {
	width: 30%;
	cursor: pointer;
	left: 0;
}
#prev:hover {
	background-image: linear-gradient(to right, rgba(0, 0, 0, 0.3) , transparent);
}
#next {
	width: 30%;
	cursor: pointer;
	right: 0;
}
#next:hover {
	background-image: linear-gradient(to left, rgba(0, 0, 0, 0.3) , transparent);
}
</style>
<form method="post" target="iframeStats" action="/chess/{{ .Key }}/stats">
	<input type="hidden" name="csrf" value="{{ .CSRF }}" />
	<input type="hidden" name="move_idx" value="{{ .MoveIdx }}" />
	<div style="position: fixed; top: 0; left: 0; right: 0; bottom: 0;">
		<button name="btn_submit" value="prev_position" type="submit" id="prev"></button>
		<button name="btn_submit" value="next_position" type="submit" id="next"></button>
	</div>
</form>`

		data := map[string]any{
			"CSRF":    csrf,
			"MoveIdx": -1,
			"Key":     key,
		}

		var buf bytes.Buffer
		_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)

		return c.HTML(http.StatusOK, buf.String())
	}

	if c.Request().Method == http.MethodPost {
		if !g.IsPlayer(authUser.ID) {
			return hutils.RedirectReferer(c)
		}

		btnSubmit := c.Request().PostFormValue("btn_submit")
		if btnSubmit == "resign-cancel" {
			return hutils.RedirectReferer(c)

		} else if btnSubmit == "resign" {

			htmlTmpl := `<form method="post">
	<input type="hidden" name="csrf" value="{{ .CSRF }}" />
	<div style="position: fixed; top: calc(50% - 80px); left: calc(50% - 100px); width: 200px; height: 80px; background-color: #444; border-radius: 5px;">
		<div style="padding: 10px;">
			<span style="margin-bottom: 5px; display: block; color: #eee;">Confirm resign:</span>
			<button type="submit" name="btn_submit" value="resign-confirm" style="background-color: #aaa;">Confirm resign</button>
			<button type="submit" name="btn_submit" value="resign-cancel" style="background-color: #aaa;">Cancel</button>
		</div>
	</div>
</form>`

			data := map[string]any{
				"CSRF": csrf,
			}

			var buf bytes.Buffer
			_ = utils.Must(template.New("").Parse(htmlTmpl)).Execute(&buf, data)

			return c.HTML(http.StatusOK, buf.String())

		} else if btnSubmit == "resign-confirm" {
			resignColor := utils.Ternary(isFlipped, chess.Black, chess.White)
			game.Resign(resignColor)
			g.DbChessGame.PGN = game.String()
			g.DbChessGame.Outcome = game.Outcome().String()
			g.DbChessGame.DoSave(db)
			interceptors.ChessPubSub.Pub(key, interceptors.ChessMove{})

		} else {
			if err := interceptors.ChessInstance.SendMove(key, authUser.ID, g, c); err != nil {
				logrus.Error(err)
			}
		}
		return hutils.RedirectReferer(c)
	}

	htmlTmpl := hutils.HtmlCssReset + interceptors.ChessCSS + `
<form method="post">
	<input type="hidden" name="csrf" value="{{ .CSRF }}" />
	<input type="hidden" name="move_idx" value="{{ .MoveIdx }}" />
	<table class="newBoard">
		{{ range $row := .Rows }}
			<tr>
				{{ range $col := $.Cols }}
					{{ $id := GetID $row $col }}
					<td>
						<input name="sq_{{ $id }}" id="sq_{{ $id }}" type="checkbox" value="1" />
						<label for="sq_{{ $id }}"></label>
					</td>
				{{ end }}
			</tr>
		{{ end }}
	</table>
	<div style="width: 100%; display: flex; margin: 5px 0;">
		<div><button type="submit" name="btn_submit" style="background-color: #aaa;">Move</button></div>
		<div>
			<span style="color: #aaa; margin-left: 20px;">Promo:</span>
			<select name="promotion" style="background-color: #aaa;">
				<option value="queen">Queen</option>
				<option value="rook">Rook</option>
				<option value="knight">Knight</option>
				<option value="bishop">Bishop</option>
			</select>
		</div>
		<div style="margin-left: auto;">
			<button type="submit" name="btn_submit" value="resign" style="background-color: #aaa; margin-left: 50px;">Resign</button>
		</div>
	</div>
</form>`

	data := map[string]any{
		"Rows":    []int{0, 1, 2, 3, 4, 5, 6, 7},
		"Cols":    []int{0, 1, 2, 3, 4, 5, 6, 7},
		"Key":     key,
		"CSRF":    csrf,
		"MoveIdx": len(g.Game.Moves()),
	}

	fns := template.FuncMap{
		"GetID": func(row, col int) int { return interceptors.GetID(row, col, isFlipped) },
	}

	var buf bytes.Buffer
	_ = utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf, data)

	return c.HTML(http.StatusOK, buf.String())
}

func squareCoord(sq chess.Square, isFlipped bool) (int, int) {
	x, y := int(sq.File()), int(sq.Rank())
	if isFlipped {
		x = 7 - x
	} else {
		y = 7 - y
	}
	return x, y
}

func initPiecesCache(game *chess.Game) map[string]chess.Square {
	piecesCache := make(map[string]chess.Square)
	pos := game.Positions()[0]
	for i := 0; i < 64; i++ {
		sq := chess.Square(i)
		if pos.Board().Piece(sq) != chess.NoPiece {
			piecesCache["piece_"+sq.String()] = sq
		}
	}
	return piecesCache
}

const animationMs = 400

func animate(s1, s2 chess.Square, id string, isFlipped bool, animationIdx *int, styles *StylesBuilder) {
	x1, y1 := squareCoord(s1, isFlipped)
	x2, y2 := squareCoord(s2, isFlipped)
	*animationIdx++
	animationName := fmt.Sprintf("move_anim_%d", *animationIdx)
	keyframes := "@keyframes %s {" +
		"from { left: calc(%d*12.5%%); top: calc(%d*12.5%%); }" +
		"  to { left: calc(%d*12.5%%); top: calc(%d*12.5%%); } }\n"
	styles.Appendf(keyframes, animationName, x1, y1, x2, y2)
	styles.Appendf("#%s { animation: %s %dms forwards; }\n", id, animationName, animationMs)
}

func ChessGameHandler(c echo.Context) error {
	debugChess := true

	authUser := c.Get("authUser").(*database.User)
	key := c.Param("key")

	g, _ := interceptors.ChessInstance.GetGame(key)
	if g == nil {
		if debugChess && config.Development.IsTrue() {
			// Chess debug
			db := c.Get("database").(*database.DkfDB)
			user1, _ := db.GetUserByID(1)
			user2, _ := db.GetUserByID(30814)
			if _, err := interceptors.ChessInstance.NewGame(key, user1, user2); err != nil {
				logrus.Error(err)
				return c.Redirect(http.StatusFound, "/")
			}
			var err error
			g, err = interceptors.ChessInstance.GetGame(key)
			if err != nil {
				logrus.Error(err)
				return c.Redirect(http.StatusFound, "/")
			}
			g.MakeMoves(kingSideCastleGame, db)
		} else {
			return c.Redirect(http.StatusFound, "/")
		}
	}

	game := g.Game

	// Keep track of where on the board a piece was last seen for this specific http stream
	piecesCache1 := initPiecesCache(game)

	isFlipped := authUser.ID == g.Player2.ID

	isSpectator := !g.IsPlayer(authUser.ID)
	if isSpectator && c.QueryParam("r") != "" {
		isFlipped = true
	}

	//isYourTurnFn := func() bool {
	//	return authUser.ID == g.Player1.ID && game.Position().Turn() == chess.White ||
	//		authUser.ID == g.Player2.ID && game.Position().Turn() == chess.Black
	//}
	//isYourTurn := isYourTurnFn()

	send := func(s string) {
		_, _ = c.Response().Write([]byte(s))
	}

	// Keep track of "if the game was over" when we loaded the page
	gameLoadedOver := game.Outcome() != chess.NoOutcome

	streamItem, err := stream.SetStreaming(c, authUser.ID, key)
	if err != nil {
		return nil
	}
	defer streamItem.Cleanup()

	send(hutils.HtmlCssReset)
	send(`<style>html, body { background-color: #222; }</style>`)

	authorizedChannels := make([]string, 0)
	authorizedChannels = append(authorizedChannels, key)
	authorizedChannels = append(authorizedChannels, key+"_"+authUser.Username.String())

	sub := interceptors.ChessPubSub.Subscribe(authorizedChannels)
	defer sub.Close()

	var card1 string
	if isSpectator {
		card1 = g.DrawSpectatorCard(0, key, isFlipped, authUser.ChessSoundsEnabled, authUser.CanUseChessAnalyze)
	} else {
		card1 = g.DrawPlayerCard(0, key, isFlipped, false, authUser.ChessSoundsEnabled, authUser.CanUseChessAnalyze)
	}
	send(card1)

	go func(c echo.Context, key string, p1ID, p2ID database.UserID) {
		p1Online := false
		p2Online := false
		var once utils.Once
		for {
			select {
			case <-once.After(100 * time.Millisecond):
			case <-time.After(5 * time.Second):
			case <-streamItem.Quit:
				return
			}
			p1Count := usersStreamsManager.Inst.GetUserStreamsCountFor(p1ID, key)
			p2Count := usersStreamsManager.Inst.GetUserStreamsCountFor(p2ID, key)
			if p1Online && p1Count == 0 {
				p1Online = false
				send(`<style>#p1Status { background-color: darkred !important; }</style>`)
			} else if !p1Online && p1Count > 0 {
				p1Online = true
				send(`<style>#p1Status { background-color: green !important; }</style>`)
			}
			if p2Online && p2Count == 0 {
				p2Online = false
				send(`<style>#p2Status { background-color: darkred !important; }</style>`)
			} else if !p2Online && p2Count > 0 {
				p2Online = true
				send(`<style>#p2Status { background-color: green !important; }</style>`)
			}
			c.Response().Flush()
		}
	}(c, key, g.Player1.ID, g.Player2.ID)

	var animationIdx int
Loop:
	for {
		select {
		case <-streamItem.Quit:
			break Loop
		default:
		}

		// If we loaded the page and game was ongoing, we will stop the infinite loading page and display pgn
		if game.Outcome() != chess.NoOutcome && !gameLoadedOver {
			send(`<style>#outcome:after { content: "` + game.Outcome().String() + `" }</style>`)
			send(`<style>.gameover { display: none !important; }</style>`)
			send(`<div style="position: absolute; width: 200px; left: calc(50% - 100px); bottom: 20px">`)
			send(`<textarea readonly>` + game.String() + `</textarea>`)
			if authUser.CanUseChessAnalyze {
				send(`<a style="color: #eee;" href="/chess/` + key + `/analyze">Analyse</a>`)
			}
			send(`</div>`)
			break
		}

		_, payload, err := sub.ReceiveTimeout2(1*time.Second, streamItem.Quit)
		if err != nil {
			if errors.Is(err, pubsub.ErrCancelled) {
				break Loop
			}
			continue
		}

		// If game was over when we loaded the page
		if game.Outcome() != chess.NoOutcome && gameLoadedOver {
			moveIdx := payload.MoveIdx
			if moveIdx != 0 {
				pos := game.Positions()[moveIdx]
				moves := game.Moves()[:moveIdx]
				lastMove := moves[len(moves)-1]
				piecesCache := interceptors.InitPiecesCache(moves)
				squareMap := pos.Board().SquareMap()

				var bestMove *chess.Move
				bestMoveStr := payload.BestMove
				if bestMoveStr != "" {
					bestMove, err = chess.UCINotation{}.Decode(pos, bestMoveStr)
					if err != nil {
						logrus.Error(err)
					}
				}

				checkIDStr := ""
				if lastMove.HasTag(chess.Check) && pos.Turn() == chess.White {
					checkIDStr = interceptors.WhiteKingID
				} else if lastMove.HasTag(chess.Check) && pos.Turn() == chess.Black {
					checkIDStr = interceptors.BlackKingID
				}

				var styles StylesBuilder
				renderAdvantages(&styles, pos)
				renderHideAllPieces(&styles, piecesCache, piecesCache1, squareMap)
				renderChecks(&styles, checkIDStr)
				renderLastMove(&styles, *lastMove)
				renderBestMove(&styles, bestMove, isFlipped)
				renderShowVisiblePieceInPosition(&styles, &animationIdx, squareMap, piecesCache, piecesCache1, isFlipped)

				send(styles.Build())
				c.Response().Flush()
			}
			continue
		}

		if authUser.ChessSoundsEnabled {
			if game.Method() != chess.Resignation {
				isCapture := payload.Move.HasTag(chess.Capture) || payload.Move.HasTag(chess.EnPassant)
				audioFile := utils.Ternary(isCapture, "Capture.ogg", "Move.ogg")
				send(`<audio src="/public/sounds/chess/` + audioFile + `" autoplay></audio>`)
			}
		}

		var styles StylesBuilder

		animate(payload.Move.S1(), payload.Move.S2(), payload.IDStr1, isFlipped, &animationIdx, &styles)

		if payload.Move.Promo() != chess.NoPieceType || payload.IDStr2 != "" {
			// Ensure the capturing piece is draw above the one being captured
			if payload.IDStr2 != "" {
				styles.Appendf(`#%s { z-index: 2; }`, payload.IDStr2)
				styles.Appendf(`#%s { z-index: 3; }`, payload.IDStr1)
			}
			// Wait until end of moving animation before hiding the captured piece or change promotion image
			go func(payload interceptors.ChessMove, c echo.Context) {
				select {
				case <-time.After(animationMs * time.Millisecond):
				case <-streamItem.Quit:
					return
				}
				if payload.IDStr2 != "" {
					send(fmt.Sprintf(`<style>#%s { display: none !important; }</style>`, payload.IDStr2))
				}
				if payload.Move.Promo() != chess.NoPieceType {
					pieceColor := utils.Ternary(payload.Move.S2().Rank() == chess.Rank8, chess.White, chess.Black)
					promoImg := "/public/img/chess/" + pieceColor.String() + strings.ToUpper(payload.Move.Promo().String()) + ".png"
					send(fmt.Sprintf(`<style>#%s { background-image: url("%s") !important; }</style>`, payload.IDStr1, promoImg))
				}
				c.Response().Flush()
			}(payload, c)
		}

		// Animate rook during castle
		animateRookFn := animate
		if payload.Move.HasTag(chess.KingSideCastle) {
			if payload.Move.S1() == chess.E1 {
				animateRookFn(chess.H1, chess.F1, interceptors.WhiteKingSideRookID, isFlipped, &animationIdx, &styles)
			} else if payload.Move.S1() == chess.E8 {
				animateRookFn(chess.H8, chess.F8, interceptors.BlackKingSideRookID, isFlipped, &animationIdx, &styles)
			}
		} else if payload.Move.HasTag(chess.QueenSideCastle) {
			if payload.Move.S1() == chess.E1 {
				animateRookFn(chess.A1, chess.D1, interceptors.WhiteQueenSideRookID, isFlipped, &animationIdx, &styles)
			} else if payload.Move.S1() == chess.E8 {
				animateRookFn(chess.A8, chess.D8, interceptors.BlackQueenSideRookID, isFlipped, &animationIdx, &styles)
			}
		}
		// En passant
		if payload.EnPassant != "" {
			styles.Appendf(`#%s { display: none !important; }`, payload.EnPassant)
		}

		renderAdvantages(&styles, game.Position())
		renderLastMove(&styles, payload.Move)
		renderChecks(&styles, payload.CheckIDStr)

		send(styles.Build())

		c.Response().Flush()
	}
	return nil
}

func renderShowVisiblePieceInPosition(styles *StylesBuilder, animationIdx *int,
	squareMap map[chess.Square]chess.Piece, piecesCache map[chess.Square]string, piecesCache1 map[string]chess.Square, isFlipped bool) {
	oldSqs := hashset.New[chess.Square]()
	for newSq := range squareMap {
		sqID := piecesCache[newSq]      // Get ID of piece on square newSq
		currentSq := piecesCache1[sqID] // Get current square location of the piece
		if currentSq != newSq {
			oldSqs.Set(currentSq)
		}
	}

	for newSq, piece := range squareMap {
		sqID := piecesCache[newSq]      // Get ID of piece on square newSq
		currentSq := piecesCache1[sqID] // Get current square location of the piece
		bStyle := fmt.Sprintf("#%s { display: block !important; ", sqID)
		x, y := squareCoord(newSq, isFlipped)
		bStyle += fmt.Sprintf("left: calc(%d*12.5%%); top: calc(%d*12.5%%); animation: none; ", x, y)
		if strings.HasSuffix(sqID, "2") || strings.HasSuffix(sqID, "7") {
			bStyle += "background-image: url(/public/img/chess/" + piece.Color().String() + strings.ToUpper(piece.Type().String()) + ".png) !important; "
		}
		bStyle += "}\n"
		styles.Append(bStyle)
		if currentSq != newSq && !oldSqs.Contains(newSq) {
			animate(currentSq, newSq, sqID, isFlipped, animationIdx, styles) // Move piece from current square to the new square where we want it to be
		}
		piecesCache1[sqID] = newSq // Update cache of location of the piece
	}
}

func renderAdvantages(styles *StylesBuilder, pos *chess.Position) {
	whiteAdv, whiteScore, blackAdv, blackScore := interceptors.CalcAdvantage(pos)
	styles.Appendf(`#white-advantage:before { content: "%s" !important; }`, whiteAdv)
	styles.Appendf(`#white-advantage .score:after { content: "%s" !important; }`, whiteScore)
	styles.Appendf(`#black-advantage:before { content: "%s" !important; }`, blackAdv)
	styles.Appendf(`#black-advantage .score:after { content: "%s" !important; }`, blackScore)
}

func renderHideAllPieces(styles *StylesBuilder, piecesCache map[chess.Square]string, piecesCache1 map[string]chess.Square, squareMap map[chess.Square]chess.Piece) {
	toHideMap := make(map[string]struct{})
	for id, _ := range piecesCache1 {
		toHideMap[id] = struct{}{}
	}
	for sq := range squareMap {
		idOnSq, _ := piecesCache[sq]
		delete(toHideMap, idOnSq)
	}
	toHide := make([]string, 0)
	for id := range toHideMap {
		toHide = append(toHide, "#"+id)
	}
	styles.Appendf(`%s { display: none !important; }`, strings.Join(toHide, ", "))
}

func calcDisc(x1, y1, x2, y2 int) (d int, isDiag, isLine bool) {
	dx := int(math.Abs(float64(x2 - x1)))
	dy := int(math.Abs(float64(y2 - y1)))
	if x1 == x2 {
		d = dy
		isLine = true
	} else if y1 == y2 {
		d = dx
		isLine = true
	} else {
		d = dx + dy
	}
	isDiag = dx == dy
	return
}

func arrow(s1, s2 chess.Square, isFlipped bool) (out string) {
	cx1, cy1 := squareCoord(s1, isFlipped)
	cx2, cy2 := squareCoord(s2, isFlipped)
	dist, isDiag, isLine := calcDisc(cx1, cy1, cx2, cy2)
	a := math.Atan2(float64(cy1-cy2), float64(cx1-cx2)) + 3*math.Pi/2
	out += fmt.Sprintf("#arrow { "+
		"display: block !important; "+
		"transform: rotate(%.9frad) !important; "+
		"top: calc(%d*12.5%% + (12.5%%/2)) !important; "+
		"left: calc(%d*12.5%% + (12.5%%/2) - 6.25%%) !important; "+
		"} ", a, cy2, cx2)
	var h string
	if isDiag {
		dist /= 2
		// sqrt(100^2 + 100^2) = 141.42
		// sqrt(30^2 + 30^2) = 42.43
		h = fmt.Sprintf("calc(%d*141.42%% + 42.43%% + 55%%)", dist-1)
	} else if isLine {
		h = fmt.Sprintf("calc(%d*100%% + 55%%)", dist-1)
	} else {
		// sqrt(100^2 + 200^2) = 223.60
		h = fmt.Sprintf("calc(223.60%% - 45%%)")
	}
	out += fmt.Sprintf("#arrow .rectangle { height: %s !important; }", h)
	return
}

func renderBestMove(styles *StylesBuilder, bestMove *chess.Move, isFlipped bool) {
	if bestMove != nil {
		s1 := bestMove.S1()
		s2 := bestMove.S2()
		arrowStyle := arrow(s1, s2, isFlipped)
		styles.Append(arrowStyle)
	} else {
		styles.Append(`#arrow { display: none !important; }`)
	}
}

func renderLastMove(styles *StylesBuilder, lastMove chess.Move) {
	styles.Appendf(`.square { background-color: transparent !important; }`)
	styles.Appendf(`.square_%d, .square_%d { background-color: %s !important; }`,
		int(lastMove.S1()), int(lastMove.S2()), interceptors.LastMoveColor)
}

func renderChecks(styles *StylesBuilder, checkID string) {
	// Reset kings background to transparent
	styles.Appendf(`#%s, #%s { background-color: transparent !important; }`, interceptors.WhiteKingID, interceptors.BlackKingID)
	// Render "checks" red background
	if checkID != "" {
		styles.Appendf(`#%s { background-color: %s !important; }`, checkID, interceptors.CheckColor)
	}
}

func ChessAnalyzeHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	if !authUser.CanUseChessAnalyze {
		return c.Redirect(http.StatusFound, "/")
	}
	var data chessAnalyzeData
	data.Pgn = c.Request().PostFormValue("pgn")
	return c.Render(http.StatusOK, "chess-analyze", data)
}
