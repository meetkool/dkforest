package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type UserForumThreadSubscription struct {
	UserID    UserID
	ThreadID  ForumThreadID
	CreatedAt time.Time
	User      User
}

func (s *UserForumThreadSubscription) DoSave(db *DkfDB) {
	if err := db.db.Save(s).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) SubscribeToForumThread(userID UserID, threadID ForumThreadID) (err error) {
	return d.db.Create(&UserForumThreadSubscription{UserID: userID, ThreadID: threadID}).Error
}

func (d *DkfDB) UnsubscribeFromForumThread(userID UserID, threadID ForumThreadID) (err error) {
	return d.db.Delete(&UserForumThreadSubscription{}, "user_id = ? AND thread_id = ?", userID, threadID).Error
}

func (d *DkfDB) IsUserSubscribedToForumThread(userID UserID, threadID ForumThreadID) bool {
	var count int64
	d.db.Model(UserForumThreadSubscription{}).Where("user_id = ? AND thread_id = ?", userID, threadID).Count(&count)
	return count == 1
}

func (d *DkfDB) GetUsersSubscribedToForumThread(threadID ForumThreadID) (out []UserForumThreadSubscription, err error) {
	err = d.db.Preload("User").Find(&out, "thread_id = ?", threadID).Error
	return
}
