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

type chatMessagesData struct {
	IsModerator          bool
	NbButtons            int64
	Messages             []database.ChatMessage
	Members              []managers.UserInfo
	MembersInChat        []managers.UserInfo
	VisibleMemberInChat  bool // either or not at least 1 user is "visibile" (not hellbanned)
	PreventRefresh       bool
	TopBarQueryParams    string
	DateFormat           string
	RoomName             string
	InboxCount           int64
	ManualRefreshTimeout int64
	ReadMarker           database.ChatReadMarker
	OfficialRooms        []database.ChatRoomAug
	SubscribedRooms      []database.ChatRoomAug
}

func (c chatMessagesData) MarshalJSON() ([]byte, error) {
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
