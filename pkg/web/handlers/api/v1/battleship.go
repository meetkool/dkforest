package v1

import (
	"bytes"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/fogleman/gg"
	"github.com/sirupsen/logrus"
	"html/template"
	"image/color"
	"strconv"
	"sync"
	"time"
)

// Carrier 5
// Battleship 4
// Cruiser 3
// Submarine 3
// Destroyer 2

/**
◀■■▶
▲
█
▼
●
*/

var BattleshipInstance *Battleship

type BSCoordinate struct {
	x, y int
}

type BSPlayer struct {
	id        database.UserID
	username  string
	userStyle string
	card      *BSCard
	shots     map[int]struct{}
}

func newPlayer(player database.User) *BSPlayer {
	p := new(BSPlayer)
	p.id = player.ID
	p.username = player.Username
	p.userStyle = player.GenerateChatStyle()
	p.card = generateCard()
	p.shots = make(map[int]struct{})
	return p
}

func (p BSPlayer) DrawForm(enemy *BSPlayer, yourTurn, gameEnded bool) string {
	out := `<form method="post" style="margin-left: 10px;" action="/api/v1/chat/top-bar/battleship" target="iframe1">`
	for i := 0; i < 10; i++ {
		row := rune('A' + i)
		for j := 0; j < 10; j++ {
			if _, ok := p.shots[i*10+j]; ok {
				if enemy.card.hasShipAt(i*10 + j) {
					out += `<button style="height: 15px; width: 10px; background-color: red;" name="message" value="" disabled></button>`
				} else {
					out += `<button style="height: 15px; width: 10px; background-color: yellow;" name="message" value="" disabled></button>`
				}
			} else {
				if yourTurn && !gameEnded {
					out += `<button style="height: 15px; width: 10px;" name="message" value="/pm ` + enemy.username + " /bs " + string(row) + strconv.Itoa(j) + `"></button>`
				} else {
					out += `<button style="height: 15px; width: 10px;" name="message" value="/pm ` + enemy.username + " /bs " + string(row) + strconv.Itoa(j) + `" disabled></button>`
				}
			}
		}
		out += `<br />`
	}
	out += `</form>`
	return out
}

type Direction int

const (
	vertical Direction = iota + 1
	horizontal
)

type BSShip struct {
	name       string
	x, y, size int
	direction  Direction
	reversed   bool
	health     int
}

func newShip(name string, x, y, size int, dir Direction, reversed bool) BSShip {
	return BSShip{name: name, x: x, y: y, size: size, direction: dir, health: size, reversed: reversed}
}

func (s BSShip) contains(pos int) bool {
	for _, el := range s.getPos() {
		if pos == el {
			return true
		}
	}
	return false
}

func (s BSShip) getPos() (out []int) {
	for i := 0; i < s.size; i++ {
		incr := i
		if s.direction == vertical {
			incr *= 10
		}
		out = append(out, s.y*10+s.x+incr)
	}
	return
}

type BSCard struct {
	carrier    BSShip
	battleShip BSShip
	cruiser    BSShip
	submarine  BSShip
	destroyer  BSShip
}

func (c *BSCard) collide(newShip BSShip) bool {
	for _, p := range newShip.getPos() {
		if c.carrier.contains(p) ||
			c.battleShip.contains(p) ||
			c.cruiser.contains(p) ||
			c.submarine.contains(p) ||
			c.destroyer.contains(p) {
			return true
		}
	}
	return false
}

func (c *BSCard) shot(pos int) {
	if c.carrier.contains(pos) {
		c.carrier.health -= 1
	} else if c.battleShip.contains(pos) {
		c.battleShip.health -= 1
	} else if c.cruiser.contains(pos) {
		c.cruiser.health -= 1
	} else if c.submarine.contains(pos) {
		c.submarine.health -= 1
	} else if c.destroyer.contains(pos) {
		c.destroyer.health -= 1
	}
}

func (c BSCard) allShipsDead() bool {
	return c.carrier.health == 0 &&
		c.battleShip.health == 0 &&
		c.cruiser.health == 0 &&
		c.submarine.health == 0 &&
		c.destroyer.health == 0
}

func (c BSCard) shipAt(pos int) (string, bool) {
	if c.carrier.contains(pos) {
		return "carrier", c.carrier.health == 0
	} else if c.battleShip.contains(pos) {
		return "battleShip", c.battleShip.health == 0
	} else if c.cruiser.contains(pos) {
		return "cruiser", c.cruiser.health == 0
	} else if c.submarine.contains(pos) {
		return "submarine", c.submarine.health == 0
	} else if c.destroyer.contains(pos) {
		return "destroyer", c.destroyer.health == 0
	}
	return "", false
}

func (c BSCard) hasShipAt(pos int) bool {
	var allPos []int
	allPos = append(allPos, c.carrier.getPos()...)
	allPos = append(allPos, c.battleShip.getPos()...)
	allPos = append(allPos, c.cruiser.getPos()...)
	allPos = append(allPos, c.submarine.getPos()...)
	allPos = append(allPos, c.destroyer.getPos()...)
	for _, el := range allPos {
		if pos == el {
			return true
		}
	}
	return false
}

type BSGame struct {
	lastUpdated time.Time
	turn        int
	player1     *BSPlayer
	player2     *BSPlayer
}

func newGame(player1, player2 database.User) *BSGame {
	g := new(BSGame)
	g.lastUpdated = time.Now()
	g.player1 = newPlayer(player1)
	g.player2 = newPlayer(player2)
	return g
}

func (g BSGame) IsPlayerTurn(playerID database.UserID) bool {
	return g.turn == 0 && g.player1.id == playerID ||
		g.turn == 1 && g.player2.id == playerID
}

func (g *BSGame) Shot(pos string) (shipStr string, shipDead, gameEnded bool, err error) {
	g.lastUpdated = time.Now()
	rowStr := pos[0]
	row := int(rowStr - 'A')
	col, _ := strconv.Atoi(string(pos[1]))
	p := row*10 + col

	ent1 := g.player1
	ent2 := g.player2
	if g.turn == 1 {
		ent1, ent2 = ent2, ent1
	}
	if _, ok := ent1.shots[p]; ok {
		return "", false, false, errors.New("position already hit")
	}
	ent1.shots[p] = struct{}{}
	ent2.card.shot(p)
	shipStr, shipDead = ent2.card.shipAt(p)
	gameEnded = ent2.card.allShipsDead()

	g.turn = (g.turn + 1) % 2
	return
}

type Battleship struct {
	sync.Mutex
	zeroID database.UserID
	games  map[string]*BSGame
}

func NewBattleship() *Battleship {
	zeroUser, _ := database.GetUserByUsername(config.NullUsername)
	b := &Battleship{zeroID: zeroUser.ID}
	b.games = make(map[string]*BSGame)

	// Thread that cleanup inactive games
	go func() {
		for {
			time.Sleep(time.Minute)
			b.Lock()
			for k, g := range b.games {
				if time.Since(g.lastUpdated) > 5*time.Minute {
					delete(b.games, k)
				}
			}
			b.Unlock()
		}
	}()

	return b
}

func generateCard() *BSCard {
	c := new(BSCard)
	genTmpShip := func(name string, size int) (out BSShip) {
		reversed := utils.RandBool()
		dir := utils.RandChoice([]Direction{horizontal, vertical})
		val1 := utils.RandInt(0, 9)
		val2 := utils.RandInt(0, 9-size)
		if dir == horizontal {
			val1, val2 = val2, val1
		}
		out = newShip(name, val1, val2, size, dir, reversed)
		return
	}
	for _, i := range []int{0, 1, 2, 3, 4} { // iterate 5 times (for each boat)
		names := []string{"carrier", "battleship", "cruiser", "submarine", "destroyer"}
		sizes := []int{5, 4, 3, 3, 2} // respective boat size
		for {
			tmpShip := genTmpShip(names[i], sizes[i])
			// If boat collide with another boat, we need to generate a new position for that boat
			if c.collide(tmpShip) {
				continue
			}
			// boat position is valid, assign it
			switch i {
			case 0:
				c.carrier = tmpShip
			case 1:
				c.battleShip = tmpShip
			case 2:
				c.cruiser = tmpShip
			case 3:
				c.submarine = tmpShip
			case 4:
				c.destroyer = tmpShip
			}
			break
		}
	}
	return c
}

func (g *BSGame) drawCardFor(tmp int, isNewGame, shipDead, gameEnded bool, shipStr, pos string) (out string) {
	you := g.player1
	enemy := g.player2
	if tmp == 1 {
		you = g.player2
		enemy = g.player1
	}

	imgB64Fn := func(myCard bool) string {
		ent1 := enemy
		ent2 := you
		if myCard {
			ent1 = you
			ent2 = enemy
		}

		c := gg.NewContext(177, 177)

		c.Push()
		c.SetColor(color.White)
		c.DrawRectangle(0, 0, 177, 177)
		c.Fill()
		c.Pop()

		c.Push()
		c.SetColor(color.Black)
		x := 22.0
		y := 13.0
		c.DrawString("0", x, y)
		c.DrawString("1", x+16, y)
		c.DrawString("2", x+16+16, y)
		c.DrawString("3", x+16+16+16, y)
		c.DrawString("4", x+16+16+16+16, y)
		c.DrawString("5", x+16+16+16+16+16, y)
		c.DrawString("6", x+16+16+16+16+16+16, y)
		c.DrawString("7", x+16+16+16+16+16+16+16, y)
		c.DrawString("8", x+16+16+16+16+16+16+16+16, y)
		c.DrawString("9", x+16+16+16+16+16+16+16+16+16, y)
		x = 6
		y = 29.0
		c.DrawString("A", x, y)
		c.DrawString("B", x, y+16)
		c.DrawString("C", x, y+16+16)
		c.DrawString("D", x, y+16+16+16)
		c.DrawString("E", x, y+16+16+16+16)
		c.DrawString("F", x, y+16+16+16+16+16)
		c.DrawString("G", x, y+16+16+16+16+16+16)
		c.DrawString("H", x, y+16+16+16+16+16+16+16)
		c.DrawString("I", x, y+16+16+16+16+16+16+16+16)
		c.DrawString("J", x, y+16+16+16+16+16+16+16+16+16)
		c.Pop()

		c.Push()
		c.SetLineWidth(1)
		c.SetColor(color.RGBA{R: 90, G: 90, B: 90, A: 255})
		for col := 0.0; col < 12; col++ {
			c.MoveTo(0.5+col*16, 0)
			c.LineTo(0.5+col*16, 176)
			c.Stroke()
		}
		for row := 0.0; row < 12; row++ {
			c.MoveTo(0, 0.5+row*16)
			c.LineTo(176, 0.5+row*16)
			c.Stroke()
		}
		c.Pop()

		drawShip := func(s BSShip) {
			if !myCard && s.health != 0 && !gameEnded {
				return
			}
			//fmt.Println(s.name, s.x, s.y, s.direction, s.reversed)
			c.Push()
			c.Translate(0.5, 0.5)
			c.Translate(16, 16)
			c.Translate(float64(s.x)*16, float64(s.y)*16)
			if s.direction == horizontal {
				if s.reversed {
					c.Translate(float64(s.size)*16, 0)
					c.Rotate(gg.Radians(90))
				} else {
					c.Translate(0, 16)
					c.Rotate(gg.Radians(-90))
				}
			} else {
				if s.reversed {
					c.Translate(16, float64(s.size)*16)
					c.Rotate(gg.Radians(180))
				}
			}
			// Front of the ship
			c.MoveTo(1, 11)
			c.QuadraticTo(8, -10, 15, 11)
			// Length of the ship
			c.Translate(0, float64(s.size-1)*16)
			// back of the ship
			c.LineTo(15, 11)
			c.QuadraticTo(8, 17, 1, 11)
			c.ClosePath()
			if s.health == 0 {
				c.SetColor(color.RGBA{R: 100, G: 100, B: 100, A: 200})
				c.Fill()
			} else if !myCard && gameEnded {
				c.SetColor(color.RGBA{R: 100, G: 130, B: 100, A: 200})
				c.Fill()
			} else {
				c.SetColor(color.RGBA{R: 100, G: 100, B: 100, A: 255})
				c.Fill()
			}
			c.Pop()
		}
		drawShip(ent1.card.carrier)
		drawShip(ent1.card.battleShip)
		drawShip(ent1.card.cruiser)
		drawShip(ent1.card.submarine)
		drawShip(ent1.card.destroyer)

		c.Push()
		c.Translate(0.5, 0.5)
		c.Translate(16, 16)
		for shot := range ent2.shots {
			shotRow := shot / 10
			shotCol := shot % 10
			c.Push()
			c.Translate(float64(shotCol)*16, float64(shotRow)*16)

			if ent1.card.hasShipAt(shot) {
				c.DrawCircle(8, 8, 4)
				c.SetColor(color.RGBA{R: 255, G: 200, B: 0, A: 255})
				c.Fill()

				c.DrawCircle(8, 8, 3)
				c.SetColor(color.RGBA{R: 255, G: 0, B: 0, A: 255})
				c.Fill()
			} else {
				c.DrawCircle(8, 8, 3)
				c.SetColor(color.RGBA{R: 60, G: 200, B: 0, A: 255})
				c.Fill()

				c.DrawCircle(8, 8, 2)
				c.SetColor(color.RGBA{R: 100, G: 0, B: 0, A: 255})
				c.Fill()
			}

			c.Pop()
		}
		c.Pop()

		var buf bytes.Buffer
		_ = c.EncodePNG(&buf)
		imgB64 := base64.StdEncoding.EncodeToString(buf.Bytes())
		return imgB64
	}

	imgB64 := imgB64Fn(true)
	img1B64 := imgB64Fn(false)

	htmlTmpl := `
Against <span {{ .EnemyUserStyle | HTMLAttr }}>@{{ .EnemyUsername }}</span><br />
{{ if not .IsNewGame }}
	{{ if .YourTurn }}
		<span {{ .EnemyUserStyle | HTMLAttr }}>@{{ .EnemyUsername }}</span> played {{ .Pos }}
	{{ else }}
		you played {{ .Pos }}
	{{ end }}
	;
	{{ if .ShipStr }}
		{{ .ShipStr }} hit
		{{ if .ShipDead }}
 			and sunk
		{{ end }}
	{{ else }}
		miss
	{{ end }}
	;
{{ end }}
{{ if .GameEnded }}
	{{ if .YourTurn }}
		You lost!<br />
	{{ else }}
		You win!<br />
	{{ end }}
{{ else }}
	{{ if .YourTurn }}
		now is your turn<br />
	{{ else }}
		waiting for opponent<br />
	{{ end }}
{{ end }}
<table>
	<tr>
		<td><img src="data:image/png;base64,{{ .ImgB64 }}" alt="" /></td>
		<td style="vertical-align: top;">
			<form method="post" style="margin-left: 10px;" action="/api/v1/chat/top-bar/battleship" target="iframe1">
				<table style="width: 177px; height: 177px; background-image: url(data:image/png;base64,{{ .Img1B64 }})">
					<tr style="height: 16px;"><td colspan="11">&nbsp;</td></tr>
					{{ range $row := .Rows }}
						<tr style="height: 16px;">
							<td style="width: 16px;"></td>
							{{ range $col := $.Cols }}
								{{ if NotShot $row $col }}
									{{ if and $.YourTurn (not $.GameEnded) }}
										<td style="width: 16px;">
											<button style="height: 15px; width: 15px;" name="message" value="/pm {{ $.EnemyUsername }} /bs {{ GetRune $row }}{{ $col }}"></button>
										</td>
									{{ else }}
										<td style="width: 16px;"></td>
									{{ end }}
								{{ else }}
									<td style="width: 16px;"></td>
								{{ end }}
							{{ end }}
						</tr>
					{{ end }}
				</table>
			</form>
		</td>
	</tr>
</table>
`
	data := map[string]any{
		"EnemyUserStyle": enemy.userStyle,
		"EnemyUsername":  enemy.username,
		"IsNewGame":      isNewGame,
		"YourTurn":       g.turn == tmp,
		"Pos":            pos,
		"ShipStr":        shipStr,
		"ShipDead":       shipDead,
		"GameEnded":      gameEnded,
		"ImgB64":         imgB64,
		"Img1B64":        img1B64,
		"Rows":           []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		"Cols":           []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	}
	fns := template.FuncMap{
		"GetRune": func(i int) string {
			return string(rune('A' + i))
		},
		"NotShot": func(i, j int) bool {
			_, ok := you.shots[i*10+j]
			return !ok
		},
		"HTMLAttr": func(in string) template.HTMLAttr {
			return template.HTMLAttr(in)
		},
	}
	var buf bytes.Buffer
	_ = utils.Must(template.New("").Funcs(fns).Parse(htmlTmpl)).Execute(&buf, data)
	return buf.String()
}

func (b *Battleship) InterceptMsg(cmd *Command) {
	b.Lock()
	defer b.Unlock()
	m := bsRgx.FindStringSubmatch(cmd.message)
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

	var shipStr string
	var isNewGame, shipDead, gameEnded bool
	g, ok := b.games[gameKey]
	if ok {
		if !g.IsPlayerTurn(cmd.fromUserID) {
			cmd.err = errors.New("not your turn")
			return
		}
		shipStr, shipDead, gameEnded, err = g.Shot(pos)
		if err != nil {
			cmd.err = err
			return
		}
	} else {
		if pos != "" {
			cmd.err = errors.New("no Game ongoing")
			return
		}
		g = newGame(user, *cmd.authUser)
		b.games[gameKey] = g
		isNewGame = true
	}

	// Delete old messages sent by "0" to the players
	if err := database.DB.
		Where("room_id = ? AND user_id = ? AND (to_user_id = ? OR to_user_id = ?)", cmd.room.ID, b.zeroID, g.player1.id, g.player2.id).
		Delete(&database.ChatMessage{}).Error; err != nil {
		logrus.Error(err)
	}

	card1 := g.drawCardFor(0, isNewGame, shipDead, gameEnded, shipStr, pos)
	_, _ = database.CreateMsg(card1, card1, cmd.roomKey, cmd.room.ID, b.zeroID, &g.player1.id)

	card2 := g.drawCardFor(1, isNewGame, shipDead, gameEnded, shipStr, pos)
	_, _ = database.CreateMsg(card2, card2, cmd.roomKey, cmd.room.ID, b.zeroID, &g.player2.id)

	if gameEnded {
		delete(b.games, gameKey)
	}

	cmd.dataMessage = "/pm " + user.Username + " "
	cmd.err = ErrStop
}
