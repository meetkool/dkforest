package database

import (
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type UserForumThreadSubscription struct {
	gorm.Model
	UserID    UserID     `gorm:"index;not null" json:"user_id"`
	ThreadID  ForumThreadID `gorm:"index;not null" json:"thread_id"`
	User      User       `json:"user,omitempty"`
}

func (s *UserForumThreadSubscription) Save(db *DkfDB) error {
	return db.db.Save(s).Error
}

func (d *DkfDB) SubscribeToForumThread(userID UserID, threadID ForumThreadID) error {
	subscription := UserForumThreadSubscription{UserID: userID, ThreadID: threadID}
	return d.db.Create(&subscription).Error
}

func (d *DkfDB) UnsubscribeFromForumThread(userID UserID, threadID ForumThreadID) error {
	return d.db.Delete(&UserForumThreadSubscription{}, "user_id = ? AND thread_id = ?", userID, threadID).Error
}

func (d *DkfDB) IsUserSubscribedToForumThread(userID UserID, threadID ForumThreadID) bool {
	var count int64
	d.db.Model(UserForumThreadSubscription{}).Where("user_id = ? AND thread_id = ?", userID, threadID).Count(&count)
	return count == 1
}

func (d *DkfDB) GetUsersSubscribedToForumThread(threadID ForumThreadID) ([]UserForumThreadSubscription, error) {
	var subscriptions []UserForumThreadSubscription
	return subscriptions, d.db.Preload("User").Find(&subscriptions, "thread_id = ?", threadID).Error
}

