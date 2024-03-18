package database

import "time"

type KarmaHistory struct {
	ID          int64       `gorm:"column:id;primary_key;auto_increment" json:"id"`
	Karma       int64       `gorm:"column:karma" json:"karma"`
	Description string      `gorm:"column:description" json:"description"`
	UserID      UserID      `gorm:"column:user_id" json:"user_id"`
	FromUserID  *int64      `gorm:"column:from_user_id" json:"from_user_id"`
	CreatedAt   time.Time   `gorm:"column:created_at" json:"created_at"`
}

func (d *DkfDB) CreateKarmaHistory(karma int64, description string, userID UserID, fromUserID *int64) (out *KarmaHistory, err error) {
	out = &KarmaHistory{
		Karma:       karma,
	
