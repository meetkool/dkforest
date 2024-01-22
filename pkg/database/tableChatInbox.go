package database

import (
	"time"

	"github.com/sirupsen/logrus"
)

type ChatInboxMessage struct {
	ID            int64
	Message       string
	RoomID        RoomID
	UserID        UserID
	ToUserID      UserID
	ChatMessageID *int64
	IsRead        bool
	IsPm          bool
	Moderators    bool
	CreatedAt     time.Time
	User          User
	ToUser        User
	Room          ChatRoom
}

func (d *DkfDB) DeleteOldChatInboxMessages() {
	if err := d.db.Delete(ChatInboxMessage{}, "created_at < date('now', '-90 Day')").Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetUserChatInboxMessages(userID UserID) (msgs []ChatInboxMessage, err error) {
	err = d.db.Order("id DESC").
		Limit(50).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Find(&msgs, "to_user_id = ?", userID).Error
	var ids []int64
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	if err := d.db.Model(&ChatInboxMessage{}).Where("id IN (?)", ids).UpdateColumn("is_read", true).Error; err != nil {
		logrus.Error(err)
	}
	return
}

func (d *DkfDB) GetUserChatInboxMessagesSent(userID UserID) (msgs []ChatInboxMessage, err error) {
	err = d.db.Order("id DESC").
		Limit(50).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Find(&msgs, "user_id = ?", userID).Error
	return
}

func (d *DkfDB) DeleteChatInboxMessageByID(messageID int64) error {
	return d.db.Where("id = ?", messageID).Delete(&ChatInboxMessage{}).Error
}

func (d *DkfDB) DeleteChatInboxMessageByChatMessageID(chatMessageID int64) error {
	return d.db.Where("chat_message_id = ?", chatMessageID).Delete(&ChatInboxMessage{}).Error
}

func (d *DkfDB) DeleteAllChatInbox(userID UserID) error {
	return d.db.Where("to_user_id = ?", userID).Delete(&ChatInboxMessage{}).Error
}

func (d *DkfDB) DeleteUserChatInboxMessages(userID UserID) error {
	return d.db.Where("user_id = ?", userID).Delete(&ChatInboxMessage{}).Error
}

func (d *DkfDB) CreateInboxMessage(msg string, roomID RoomID, fromUserID, toUserID UserID, isPm, moderators bool, msgID *int64) {
	inbox := ChatInboxMessage{Message: msg, RoomID: roomID, UserID: fromUserID, ToUserID: toUserID, IsPm: isPm, Moderators: moderators, ChatMessageID: msgID}
	if err := d.db.Create(&inbox).Error; err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetUserInboxMessagesCount(userID UserID) (count int64) {
	d.db.Table("chat_inbox_messages").Where("to_user_id = ? AND is_read = ?", userID, false).Count(&count)
	return
}
