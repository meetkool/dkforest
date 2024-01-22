package database

import (
	"errors"
	"github.com/asaskevich/govalidator"
	"time"
)

type UserPublicNote struct {
	ID        int64
	UserID    UserID
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (d *DkfDB) GetUserPublicNotes(userID UserID) (out UserPublicNote, err error) {
	err = d.db.First(&out, "user_id = ?", userID).Error
	return
}

func (d *DkfDB) SetUserPublicNotes(userID UserID, notes string) error {
	if !govalidator.RuneLength(notes, "0", "10000") {
		return errors.New("notes must have 10000 characters maximum")
	}
	n := UserPublicNote{UserID: userID}
	if err := d.db.FirstOrCreate(&n, "user_id = ?", userID).Error; err != nil {
		return err
	}
	n.Notes = notes
	return d.db.Save(&n).Error
}
