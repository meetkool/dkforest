package database

import (
	"errors"
	"github.com/asaskevich/govalidator"
	"time"
)

type UserPrivateNote struct {
	ID        int64
	UserID    UserID
	Notes     EncryptedString
	CreatedAt time.Time
	UpdatedAt time.Time
}

func GetUserPrivateNotes(userID UserID) (out UserPrivateNote, err error) {
	err = DB.First(&out, "user_id = ?", userID).Error
	return
}

func SetUserPrivateNotes(userID UserID, notes string) error {
	if !govalidator.RuneLength(notes, "0", "10000") {
		return errors.New("notes must have 10000 characters maximum")
	}
	n := UserPrivateNote{UserID: userID}
	if err := DB.FirstOrCreate(&n, "user_id = ?", userID).Error; err != nil {
		return err
	}
	n.Notes = EncryptedString(notes)
	return DB.Save(&n).Error
}
