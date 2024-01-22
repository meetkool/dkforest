package stream

import (
	"dkforest/pkg/database"
	"dkforest/pkg/web/handlers/usersStreamsManager"
	hutils "dkforest/pkg/web/handlers/utils"
	"github.com/labstack/echo"
)

type Item struct {
	Quit <-chan struct{}
	item *usersStreamsManager.Item
}

func (s *Item) Cleanup() {
	s.item.Cleanup()
}

func SetStreaming(c echo.Context, userID database.UserID, key string) (*Item, error) {
	// Keep track of users streams, so we can limit how many are open at one time per user
	item, err := usersStreamsManager.Inst.Add(userID, key)
	if err != nil {
		return nil, err
	}
	return &Item{Quit: hutils.SetStreaming(c), item: item}, nil
}
