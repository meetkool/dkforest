package database

import (
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"time"
)

type ChessGame struct {
	ID            int64          `gorm:"primary_key" json:"id"`
	UUID          string        `json:"uuid"`
	WhiteUserID   UserID        `json:"white_user_id"`
	BlackUserID   UserID        `json:"black_user_id"`
	PGN           string        `json:"pgn"`
	Outcome       string        `json:"outcome"`
	AccuracyWhite float64       `json:"accuracy_white"`
	AccuracyBlack float64       `json:"accuracy_black"`
	Stats         []byte        `json:"stats"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	GameOver      gorm.DeletedAt `gorm:"index" json:"game_over"`
}

func (d *DkfDB) CreateChessGame(uuid string, whiteUserID, blackUserID UserID) (*ChessGame, error) {
	chessGame := ChessGame{
		UUID:        uuid,
		WhiteUserID: whiteUserID,
	
