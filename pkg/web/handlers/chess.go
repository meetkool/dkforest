package handlers

import (
	"bytes"
	"chess"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

type chessData struct {
	Games []*chess.Game
	Error string
	Username string
	Color string
}

func ChessHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)

	data := &chessData{}
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

		game, err := interceptors.ChessInstance.NewGame1("", config.GeneralRoomID, player1, player2, data.Color)
		if err != nil {
			data.Error = err.Error()
			return c.Render(http.StatusOK, "chess", data)
		}

		return c.Redirect(http.StatusFound, "/")
	}

	return c.Render(http.StatusOK, "chess", data)
}

func ChessGameAnalyzeHandler(c echo.Context) error {
	key := c.Param("key")
	db := c.Get("database").(*database.DkfDB)
	authUser := c.Get("authUser").(*database.User)

	g, err := interceptors.ChessInstance.GetGame(key)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	game := g.Game

	if game.Outcome() == chess.NoOutcome {
		return c.String(http.StatusOK, "no outcome")
	}

	csrf, _ := c.Get("csrf").(string)

	if c.Request().Method == http.MethodGet && !g.IsAnalyzing() {
		return c.HTML(http.StatusOK, fmt.Sprintf(`
<style>html, body { background-color: #222; color: #eee; }</style>
<form method="post">
	<input type="hidden" name="csrf" value="%s" />
	Total time (15-60):
	<input type="number" name="t" value="15" min="15" max=60 />
	<button type="submit">Start analyze</button>
</form>`, csrf))
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
	htmlTmpl := `
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
	<input type="hidden" name="csrf" value="%s" />
	<table class="graph">
		<tr>
			{{ range $idx, $el := .Stats.Scores }}
			
