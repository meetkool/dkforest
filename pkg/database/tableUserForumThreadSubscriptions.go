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

func (s *UserForumThreadSubscription) DoSave() {
	if err := DB.Save(s).Error; err != nil {
		logrus.Error(err)
	}
}

func SubscribeToForumThread(userID UserID, threadID ForumThreadID) (err error) {
	return DB.Create(&UserForumThreadSubscription{UserID: userID, ThreadID: threadID}).Error
}

func UnsubscribeFromForumThread(userID UserID, threadID ForumThreadID) (err error) {
	return DB.Delete(&UserForumThreadSubscription{}, "user_id = ? AND thread_id = ?", userID, threadID).Error
}

func IsUserSubscribedToForumThread(userID UserID, threadID ForumThreadID) bool {
	var count int64
	DB.Model(UserForumThreadSubscription{}).Where("user_id = ? AND thread_id = ?", userID, threadID).Count(&count)
	return count == 1
}

func GetUsersSubscribedToForumThread(threadID ForumThreadID) (out []UserForumThreadSubscription, err error) {
	err = DB.Preload("User").Find(&out, "thread_id = ?", threadID).Error
	return
}
