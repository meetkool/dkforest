package database

import (
	"github.com/sirupsen/logrus"
	"time"
)

type ChessGame struct {
	ID            int64
	UUID          string
	WhiteUserID   UserID
	BlackUserID   UserID
	PGN           string
	Outcome       string
	AccuracyWhite float64
	AccuracyBlack float64
	Stats         []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (d *DkfDB) CreateChessGame(uuid string, whiteUserID, blackUserID UserID) (*ChessGame, error) {
	chessGame := ChessGame{
		UUID:        uuid,
		WhiteUserID: whiteUserID,
		BlackUserID: blackUserID,
		Outcome:     "*",
	}
	err := d.db.Create(&chessGame).Error
	return &chessGame, err
}

func (d *DkfDB) GetChessGame(uuid string) (*ChessGame, error) {
	out := ChessGame{}
	err := d.db.First(&out, "uuid = ?", uuid).Error
	return &out, err
}

// Save chessGame in the database
func (g *ChessGame) Save(db *DkfDB) error {
	return db.db.Save(g).Error
}

// DoSave chessGame in the database, ignore error
func (g *ChessGame) DoSave(db *DkfDB) {
	if err := g.Save(db); err != nil {
		logrus.Error(err)
	}
}
