package v1

import (
	"dkforest/pkg/database"
	"dkforest/pkg/managers"
	"encoding/json"
)

type chatTopBarData struct {
	RoomName       string
	Multiline      bool
	Message        string
	QueryParams    string
	QueryParamsMl  string
	QueryParamsNml string
	Error          string
	Success        string
	CommandsList   []string
}

type chatControlsData struct {
	RoomName        string
	IsStream        bool
	ToggleMentions  bool
	TogglePms       int64
	ChatQueryParams string
}

type ChatMenuData struct {
	InboxCount          int64
	OfficialRooms       []database.ChatRoomAug1
	SubscribedRooms     []database.ChatRoomAug1
	Members             []managers.UserInfo
	MembersInChat       []managers.UserInfo
	VisibleMemberInChat bool // either or not at least 1 user is "visible" (not hellbanned)
	RoomName            string
	TopBarQueryParams   string
	PreventRefresh      bool
}

type ChatMessagesData struct {
	ChatMenuData
	NbButtons            int64
	Messages             []database.ChatMessage
	RoomName             string
	ManualRefreshTimeout int64
	ReadMarker           database.ChatReadMarker
	ForceManualRefresh   bool
	NewMessageSound      bool
	TaggedSound          bool
	PmSound              bool
	Error                string
	ErrorTs              int64
	HideRightColumn      bool
	HideTimestamps       bool
}

func (c ChatMessagesData) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Members  []managers.UserInfo
		Messages []database.ChatMessage
	}{
		Members:  c.Members,
		Messages: c.Messages,
	})
}

type testData struct {
	NewMessageSound      bool
	TaggedSound          bool
	PmSound              bool
	InboxCount           int64
	LastMessageCreatedAt string
}
