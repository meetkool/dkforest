package database

import "github.com/sirupsen/logrus"

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
	DownloadsEnabled     bool  // either or not people can download files
	PokerWithdrawEnabled bool  // either or not poker withdraw is enabled
	CaptchaDifficulty    int64 // captcha difficulty
	PowEnabled           bool
	MoneroPrice          float64
}

// GetSettings get the saved settings from the DB
func (d *DkfDB) GetSettings() (out Settings) {
	if err := d.db.Model(Settings{}).First(&out).Error; err != nil {
		out.SignupEnabled = true
		out.SilentSelfKick = true
		out.ForumEnabled = true
		out.MaybeAuthEnabled = true
		out.DownloadsEnabled = true
		out.CaptchaDifficulty = 2
		out.MoneroPrice = 170.0
		d.db.Create(&out)
	}
	return
}

// Save the settings to DB
func (s *Settings) Save(db *DkfDB) error {
	return db.db.Save(s).Error
}

// DoSave settings in the database, ignore error
func (s *Settings) DoSave(db *DkfDB) {
	if err := s.Save(db); err != nil {
		logrus.Error(err)
	}
}
