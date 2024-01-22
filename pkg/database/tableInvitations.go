package database

import (
	"dkforest/pkg/utils"
	"github.com/sirupsen/logrus"
	"time"
)

type Invitation struct {
	ID            int64
	Token         string
	OwnerUserID   UserID
	InviteeUserID UserID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Save user in the database
func (i *Invitation) Save(db *DkfDB) error {
	return db.db.Save(i).Error
}

// DoSave user in the database, ignore error
func (i *Invitation) DoSave(db *DkfDB) {
	if err := db.db.Save(i).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) CreateInvitation(userID UserID) (out Invitation, err error) {
	out = Invitation{
		Token:         utils.GenerateToken32(),
		OwnerUserID:   userID,
		InviteeUserID: 1,
	}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) GetUnusedInvitationByToken(token string) (out Invitation, err error) {
	err = d.db.First(&out, "token = ? AND invitee_user_id == 1", token).Error
	return
}

func (d *DkfDB) GetUserInvitations(userID UserID) (out []Invitation, err error) {
	err = d.db.Find(&out, "owner_user_id = ?", userID).Error
	return
}

func (d *DkfDB) GetUserUnusedInvitations(userID UserID) (out []Invitation, err error) {
	err = d.db.Find(&out, "owner_user_id = ? AND invitee_user_id == 1", userID).Error
	return
}
