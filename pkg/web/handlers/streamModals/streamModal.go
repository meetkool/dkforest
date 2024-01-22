package streamModals

import (
	"dkforest/pkg/database"
)

type IStreamModal interface {
	// Topics returns all the topics the modal is interested in
	Topics() []string
	// Handle a stream message
	Handle(db *database.DkfDB, authUser database.IUserRenderMessage, topic, csrf string, msgTyp database.ChatMessageType, send func(string)) bool
	// Implement interceptor
	Show(database.UserID, database.RoomID, database.ChatMessageType)
	Hide(database.UserID, database.RoomID)
	Css() string
}

type StreamModal struct {
	topics []string
	idx    int
	userID database.UserID
	room   database.ChatRoom
	name   string
}

func (m *StreamModal) Topics() []string {
	return m.topics
}

func (m *StreamModal) showTopic(name string, userID database.UserID, roomID database.RoomID) string {
	return m.topic(name, "show", userID, roomID)
}

func (m *StreamModal) hideTopic(name string, userID database.UserID, roomID database.RoomID) string {
	return m.topic(name, "hide", userID, roomID)
}

func (_ *StreamModal) topic(name, action string, userID database.UserID, roomID database.RoomID) string {
	return "modal_" + name + "_" + action + "_" + userID.String() + "_" + roomID.String()
}
