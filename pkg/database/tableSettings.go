package database

import "github.com/sirupsen/logrus"

// Settings table, should always be one row
type Settings struct {
	ID                int64
	MaybeAuthEnabled  bool  // either or not unauthenticated users can access the "maybe auth" pages
	SilentSelfKick    bool  // either or not people can use the forum features
	ForumEnabled      bool  // either or not people can use the forum features
	SignupEnabled     bool  // either or not people can sign up
	SignupFakeEnabled bool  // either or not signup is faked to be enabled
	ProtectHome       bool  // ...
	HomeUsersList     bool  // ...
	ForceLoginCaptcha bool  // either or not people are forced to complete captcha at login
	DownloadsEnabled  bool  // either or not people can download files
	CaptchaDifficulty int64 // captcha difficulty
}

// GetSettings get the saved settings from the DB
func GetSettings() (out Settings) {
	if err := DB.Model(Settings{}).First(&out).Error; err != nil {
		out.SignupEnabled = true
		out.SilentSelfKick = true
		out.ForumEnabled = true
		out.MaybeAuthEnabled = true
		out.DownloadsEnabled = true
		out.CaptchaDifficulty = 2
		DB.Create(&out)
	}
	return
}

// Save the settings to DB
func (s *Settings) Save() error {
	return DB.Save(s).Error
}

// DoSave settings in the database, ignore error
func (s *Settings) DoSave() {
	if err := DB.Save(s).Error; err != nil {
		logrus.Error(err)
	}
}
