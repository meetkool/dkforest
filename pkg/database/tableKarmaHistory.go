package database

import "time"

type KarmaHistory struct {
	ID          int64
	Karma       int64
	Description string
	UserID      UserID
	FromUserID  *int64
	CreatedAt   time.Time
}

func CreateKarmaHistory(karma int64, description string, userID UserID, fromUserID *int64) (out KarmaHistory, err error) {
	out = KarmaHistory{
		Karma:       karma,
		Description: description,
		UserID:      userID,
		FromUserID:  fromUserID,
	}
	err = DB.Create(&out).Error
	return
}

func (KarmaHistory) TableName() string {
	return "karma_history"
}
