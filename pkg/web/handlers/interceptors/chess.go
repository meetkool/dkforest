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
	clip-path: polygon(0%
