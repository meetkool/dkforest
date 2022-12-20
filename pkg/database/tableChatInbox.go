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

func GetUserChatInboxMessages(userID UserID) (msgs []ChatInboxMessage, err error) {
	err = DB.Order("id DESC").
		Limit(50).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Find(&msgs, "to_user_id = ?", userID).Error
	var ids []int64
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	if err := DB.Model(&ChatInboxMessage{}).Where("id IN (?)", ids).UpdateColumn("is_read", true).Error; err != nil {
		logrus.Error(err)
	}
	return
}

func GetUserChatInboxMessagesSent(userID UserID) (msgs []ChatInboxMessage, err error) {
	err = DB.Order("id DESC").
		Limit(50).
		Preload("User").
		Preload("ToUser").
		Preload("Room").
		Find(&msgs, "user_id = ?", userID).Error
	return
}

func DeleteChatInboxMessageByID(messageID int64) error {
	return DB.Where("id = ?", messageID).Delete(&ChatInboxMessage{}).Error
}

func DeleteChatInboxMessageByChatMessageID(chatMessageID int64) error {
	return DB.Where("chat_message_id = ?", chatMessageID).Delete(&ChatInboxMessage{}).Error
}

func DeleteAllChatInbox(userID UserID) error {
	return DB.Where("to_user_id = ?", userID).Delete(&ChatInboxMessage{}).Error
}

func DeleteUserChatInboxMessages(userID UserID) error {
	return DB.Where("user_id = ?", userID).Delete(&ChatInboxMessage{}).Error
}

func CreateInboxMessage(msg string, roomID RoomID, fromUserID, toUserID UserID, isPm, moderators bool, msgID *int64) {
	inbox := ChatInboxMessage{Message: msg, RoomID: roomID, UserID: fromUserID, ToUserID: toUserID, IsPm: isPm, Moderators: moderators, ChatMessageID: msgID}
	if err := DB.Create(&inbox).Error; err != nil {
		logrus.Error(err)
	}
}

func GetUserInboxMessagesCount(userID UserID) (count int64) {
	DB.Table("chat_inbox_messages").Where("to_user_id = ? AND is_read = ?", userID, false).Count(&count)
	return
}
