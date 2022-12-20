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
func (i *Invitation) Save() error {
	return DB.Save(i).Error
}

// DoSave user in the database, ignore error
func (i *Invitation) DoSave() {
	if err := DB.Save(i).Error; err != nil {
		logrus.Error(err)
	}
}

func CreateInvitation(userID UserID) (out Invitation, err error) {
	out = Invitation{
		Token:         utils.GenerateToken32(),
		OwnerUserID:   userID,
		InviteeUserID: 1,
	}
	err = DB.Create(&out).Error
	return
}

func GetUnusedInvitationByToken(token string) (out Invitation, err error) {
	err = DB.First(&out, "token = ? AND invitee_user_id == 1", token).Error
	return
}

func GetUserInvitations(userID UserID) (out []Invitation, err error) {
	err = DB.Find(&out, "owner_user_id = ?", userID).Error
	return
}

func GetUserUnusedInvitations(userID UserID) (out []Invitation, err error) {
	err = DB.Find(&out, "owner_user_id = ? AND invitee_user_id == 1", userID).Error
	return
}
