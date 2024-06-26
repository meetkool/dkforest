package managers

import (
	"dkforest/pkg/hashset"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"dkforest/pkg/database"

	"dkforest/pkg/utils"
)

func init() {
	ActiveUsers = NewActiveUsersManager()
}

type UserInfo struct {
	UserID              database.UserID
	Username            database.Username
	Color               string
	RefreshRate         int64
	LastUpdate          time.Time
	LastActivity        *time.Time
	IsModerator         bool
	IsIncognito         bool
	IsHellbanned        bool
	AfkIndicatorEnabled bool
}

type IUserInfoUser interface {
	GetID() database.UserID
	GetUsername() database.Username
	GetRefreshRate() int64
	GetChatColor() string
	IsModerator() bool
	GetIsIncognito() bool
	GetIsHellbanned() bool
	GetAFK() bool
	GetAfkIndicatorEnabled() bool
}

func newUserInfo(user IUserInfoUser, lastActivity *time.Time) UserInfo {
	return UserInfo{
		UserID:              user.GetID(),
		Username:            user.GetUsername(),
		RefreshRate:         user.GetRefreshRate(),
		Color:               user.GetChatColor(),
		IsModerator:         user.IsModerator(),
		IsIncognito:         user.GetIsIncognito(),
		IsHellbanned:        user.GetIsHellbanned(),
		AfkIndicatorEnabled: user.GetAFK() && user.GetAfkIndicatorEnabled(),
		LastUpdate:          time.Now(),
		LastActivity:        lastActivity,
	}
}

func NewUserInfo(user IUserInfoUser) UserInfo {
	return newUserInfo(user, nil)
}

func NewUserInfoUpdateActivity(user IUserInfoUser) UserInfo {
	now := time.Now()
	return newUserInfo(user, &now)
}

func (m UserInfo) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Username database.Username
		Color    string
	}{
		Username: m.Username,
		Color:    m.Color,
	})
}

type UsersMap map[database.Username]UserInfo // Username -> UserInfo

func (m UsersMap) ToArray() []UserInfo {
	out := make([]UserInfo, len(m))
	i := 0
	for _, userInfo := range m {
		out[i] = userInfo
		i++
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastActivity != nil && out[j].LastActivity != nil {
			if out[i].LastActivity.After(*out[j].LastActivity) {
				return true
			} else if out[i].LastActivity.Before(*out[j].LastActivity) {
				return false
			}
		}
		return out[i].Username < out[j].Username
	})
	return out
}

const privateRoomKeyPrefix = "p_"

type RoomKey string

func (r RoomKey) isPrivateRoom() bool {
	return strings.HasPrefix(string(r), privateRoomKeyPrefix)
}

func getRoomKey(room database.ChatRoom) RoomKey {
	if room.IsProtected() {
		return RoomKey(fmt.Sprintf("%s%d", privateRoomKeyPrefix, room.ID))
	}
	return RoomKey(utils.FormatInt64(int64(room.ID)))
}

type ActiveUsersManager struct {
	sync.RWMutex
	activeUsers map[RoomKey]UsersMap
}

func NewActiveUsersManager() *ActiveUsersManager {
	m := new(ActiveUsersManager)
	m.activeUsers = make(map[RoomKey]UsersMap)
	return m
}

var ActiveUsers *ActiveUsersManager

func (m *ActiveUsersManager) UpdateUserInRoom(room database.ChatRoom, userInfo UserInfo) {
	if userInfo.IsIncognito {
		return
	}
	roomKey := getRoomKey(room)
	usersMap := m.getRoomUsersMap(roomKey)

	m.RLock()
	prevUserInfo := usersMap[userInfo.Username]
	m.RUnlock()
	if prevUserInfo.LastActivity == nil && userInfo.LastActivity == nil {
		now := time.Now()
		userInfo.LastActivity = &now
	} else if userInfo.LastActivity == nil {
		userInfo.LastActivity = prevUserInfo.LastActivity
	}

	m.Lock()
	usersMap[userInfo.Username] = userInfo
	m.activeUsers[roomKey] = usersMap
	m.Unlock()
}

// UpdateUserHBInRooms Update the IsHellbanned property for a user.
// This is needed to ensure the user become invisible at the same time as his messages disappears.
// Otherwise, it is possible that the message becomes HB and the user still show up in the users list.
func (m *ActiveUsersManager) UpdateUserHBInRooms(newUserInfo UserInfo) {
	m.Lock()
	for roomKey, usersMap := range m.activeUsers {
		for username, userInfo := range usersMap {
			if userInfo.UserID == newUserInfo.UserID {
				prevUserInfo := m.activeUsers[roomKey][username]
				prevUserInfo.IsHellbanned = newUserInfo.IsHellbanned
				m.activeUsers[roomKey][username] = prevUserInfo
			}
		}
	}
	m.Unlock()
}

func (m *ActiveUsersManager) getRoomUsersMap(roomKey RoomKey) UsersMap {
	emptyUsersMap := make(UsersMap)
	m.RLock()
	usersMap, found := m.activeUsers[roomKey]
	m.RUnlock()
	if !found {
		m.Lock()
		m.activeUsers[roomKey] = emptyUsersMap
		usersMap = emptyUsersMap
		m.Unlock()
	}
	return usersMap
}

func (m *ActiveUsersManager) LocateUser(target database.Username) (out []database.RoomID) {
	m.RLock()
	for roomKey, usersMap := range m.activeUsers {
		for username := range usersMap {
			if strings.ToLower(string(username)) == strings.ToLower(string(target)) {
				roomID := database.RoomID(utils.DoParseInt64(string(roomKey)))
				out = append(out, roomID)
			}
		}
	}
	m.RUnlock()
	return
}

// GetActiveUsers gets a list of all users that are in public rooms.
// We use this to display online users on the home (login) page if the feature is enabled.
func (m *ActiveUsersManager) GetActiveUsers() []UserInfo {
	activeUsers := make(UsersMap)
	m.RLock()
	defer m.RUnlock()
	for roomKey, usersMap := range m.activeUsers {
		for username, userInfo := range usersMap {
			if !roomKey.isPrivateRoom() { // Skip people who are in private rooms
				activeUsers[username] = userInfo
			}
		}
	}
	return activeUsers.ToArray()
}

func GetUserIgnoreSet(db *database.DkfDB, authUser *database.User) *hashset.HashSet[database.Username] {
	ignoredSet := hashset.New[database.Username]()
	// Only fill the ignored set if the user does not display the ignored users ("Toggle ignored" chat setting)
	// and if the user has "Hide ignored users from users lists" enabled (user setting)
	if !authUser.DisplayIgnored && authUser.HideIgnoredUsersFromList {
		ignoredUsersUsernames, _ := db.GetIgnoredUsersUsernames(authUser.ID)
		for _, ignoredUserUsername := range ignoredUsersUsernames {
			ignoredSet.Insert(ignoredUserUsername)
		}
	}
	return ignoredSet
}

func (m *ActiveUsersManager) GetRoomUsers(room database.ChatRoom, ignoredSet *hashset.HashSet[database.Username]) (inRoom, inChat []UserInfo) {
	outsideUsers := make(UsersMap)
	newRoomUsersMap := make(UsersMap)
	roomIDStr := getRoomKey(room)
	// clone managers map into local variable map
	m.RLock()
	defer m.RUnlock()
	if roomUsersMap, ok := m.activeUsers[roomIDStr]; ok {
		for username, userInfo := range roomUsersMap {
			newRoomUsersMap[username] = userInfo
		}
	}
	// Build map of users outside of current room
	for roomKey, usersMap := range m.activeUsers {
		for username, userInfo := range usersMap {
			if roomKey == roomIDStr || roomKey.isPrivateRoom() { // Skip people who are in private rooms
				continue
			}
			// Only add users if they're not already in the current room
			if _, ok := newRoomUsersMap[username]; !ok {
				outsideUsers[username] = userInfo
			}
		}
	}
	// Delete ignored users
	ignoredSet.Each(func(ignoreUsername database.Username) {
		delete(newRoomUsersMap, ignoreUsername)
		delete(outsideUsers, ignoreUsername)
	})
	inRoom = newRoomUsersMap.ToArray()
	inChat = outsideUsers.ToArray()
	return
}

// RemoveUser from active users
func (m *ActiveUsersManager) RemoveUser(userID database.UserID) {
	m.Lock()
	defer m.Unlock()
	for _, usersMap := range m.activeUsers {
		for k, v := range usersMap {
			if v.UserID == userID {
				delete(usersMap, k)
			}
		}
	}
}

func (m *ActiveUsersManager) IsUserActiveInRoom(userID database.UserID, room database.ChatRoom) (found bool) {
	m.RLock()
	defer m.RUnlock()
	usersMap, found := m.activeUsers[getRoomKey(room)]
	if !found {
		return false
	}
	for _, v := range usersMap {
		if v.UserID == userID {
			return true
		}
	}
	return false
}

func (m *ActiveUsersManager) CleanupUsersCache() {
	for {
		select {
		case <-time.After(10 * time.Second):
		}
		m.Lock()
		for _, usersMap := range m.activeUsers {
			for k, userInfo := range usersMap {
				if time.Since(userInfo.LastUpdate) > time.Duration(userInfo.RefreshRate+25)*time.Second {
					delete(usersMap, k)
				}
			}
		}
		m.Unlock()
	}
}
