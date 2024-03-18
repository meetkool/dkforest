package database

import (
	"github.com/sirupsen/logrus"
	"github.com/jinzhu/gorm"
)

// Settings table, should always be one row
type Settings struct {
	ID                   int64
	MaybeAuthEnabled     bool  // either or not unauthenticated users can access the "maybe auth" pages
	SilentSelfKick       bool  // either or not people can use the forum features
	ForumEnabled         bool  // either or not people can use the forum features
	SignupEnabled        bool  // either or not people can sign up
	SignupFakeEnabled    bool  // either or not signup is faked to be enabled
	ProtectHome          bool  // ...
	HomeUsersList        bool  // ...
	ForceLoginCaptcha    bool  // either or not people are forced to complete captcha at login
	DownloadsEnabled     bool 
