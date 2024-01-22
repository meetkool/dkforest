package usersStreamsManager

import (
	"dkforest/pkg/database"
	"errors"
	"sync"
)

const userMaxStream = 15

var ErrTooManyStreams = errors.New("too many streams")

type UserStreamsMap map[string]int64

func (m *UserStreamsMap) count() (out int64) {
	for _, v := range *m {
		out += v
	}
	return
}

// UsersStreamsManager ensure that a user doesn't have more than userMaxStream
// http long polling streams open at the same time.
// If the limit is reached, the pages will then refuse to load.
// This is to prevent a malicious user from opening unlimited amount of streams and wasting the server resources.
type UsersStreamsManager struct {
	sync.RWMutex
	m map[database.UserID]UserStreamsMap
}

func NewUsersStreamsManager() *UsersStreamsManager {
	return &UsersStreamsManager{m: make(map[database.UserID]UserStreamsMap)}
}

type Item struct {
	m      *UsersStreamsManager
	userID database.UserID
	key    string
}

func (i *Item) Cleanup() {
	i.m.Remove(i.userID, i.key)
}

func (m *UsersStreamsManager) Add(userID database.UserID, key string) (*Item, error) {
	m.Lock()
	defer m.Unlock()
	userMap, found := m.m[userID]
	if found && userMap.count() >= userMaxStream {
		return nil, ErrTooManyStreams
	}
	if !found {
		userMap = make(UserStreamsMap)
	}
	userMap[key]++
	m.m[userID] = userMap
	return &Item{m: m, userID: userID, key: key}, nil
}

func (m *UsersStreamsManager) Remove(userID database.UserID, key string) {
	m.Lock()
	defer m.Unlock()
	if userMap, found := m.m[userID]; found {
		userMap[key]--
		m.m[userID] = userMap
	}
}

func (m *UsersStreamsManager) GetUserStreamsCountFor(userID database.UserID, key string) (out int64) {
	m.RLock()
	defer m.RUnlock()
	if userMap, found := m.m[userID]; found {
		if nbStreams, found1 := userMap[key]; found1 {
			return nbStreams
		}
	}
	return
}

func (m *UsersStreamsManager) GetUsers() (out []database.UserID) {
	m.RLock()
	defer m.RUnlock()
	for userID, userMap := range m.m {
		if userMap.count() > 0 {
			out = append(out, userID)
		}
	}
	return
}

var Inst = NewUsersStreamsManager()
