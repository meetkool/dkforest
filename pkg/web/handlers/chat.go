package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	dutils "dkforest/pkg/database/utils"
	"dkforest/pkg/database/web"
	"dkforest/pkg/hashset"
	"dkforest/pkg/managers"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
)

type chatData struct {
	PowEnabled         bool
	RedRoom            bool
	Room               database.ChatRoom
	CaptchaID          string
	CaptchaImg         string
	Multiline          bool
	ChatQueryParams    string
	DisplayTutorial    bool
	TutoSecs           int
	TutoFrames         []string
	IsSubscribed       bool
	IsOfficialRoom     bool
	IsStream           bool
}

func (cd *chatData) setTutorialFrames(tutoSecs int, frames *[]string) {
	cd.TutoSecs = tutoSecs
	cd.TutoFrames = *frames
}

func RedRoomHandler(c echo.Context) error {
	return chatHandler(c, true, false)
}

func ChatHandler(c echo.Context) error {
	return chatHandler(c, false, false)
}

func ChatStreamHandler(c echo.Context) error {
	return chatHandler(c, false, true)
}

func chatHandler(c echo.Context, redRoom, stream bool) error {
	// ... (rest of the function remains the same)
}

// ... (rest of the functions remain the same)

func ChatArchiveHandler(c echo.Context) error {
	// ... (rest of the function remains the same)
}

func ChatDeleteHandler(c echo.Context) error {
	// ... (rest of the function remains the same)
}

func RoomChatSettingsHandler(c echo.Context) error {
	// ... (rest of the function remains the same)
}

func ChatCreateRoomHandler(c echo.Context) error {
	// ... (rest of the function remains the same)
}

func ChatCodeHandler(c echo.Context) error {
	// ... (rest of the function remains the same)
}

func ChatHelpHandler(c echo.Context) error {
	// ... (rest of the function remains the same)
}

func getRoomName(c echo.Context) string {
	roomName := c.Param("roomName")
	if roomName == "" {
		roomName = "general"
	}
	return roomName
}

func isAccessAllowed(room database.ChatRoom, authUser *database.User) (bool, error) {
	if authUser == nil {
		return false, nil
	}

	if room.IsProtected() && authUser.ID != room.OwnerID {
		return false, nil
	}

	return true, nil
}

func getTutorialStepDuration() int {
	// ... (rest of the function remains the same)
}

func generateCssFrames(tutoSecs int, frames *[]string, isTutorial bool) []string {
	// ... (rest of the function remains the same)
}

func createSessionCookie(token string, maxAge time.Duration) *http.Cookie {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setDisplayTutorial(room database.ChatRoom, authUser *database.User) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setIsSubscribed(db *database.DkfDB, room database.ChatRoom, authUser *database.User) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPassword(room database.ChatRoom, password string) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordError(room database.ChatRoom, err error) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordSuccess(room database.ChatRoom) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordForm(room database.ChatRoom, password string) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordFormError(err error) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordFormSuccess() {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordFormVisible(visible bool) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setRoomPasswordVisible(room database.ChatRoom, visible bool) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setCaptcha(captchaID, captchaImg string) {
	// ... (rest of the function remains the same)
}

func (cd *chatData) setCaptchaError(err error) {
	// ... (
