package database

import "github.com/sirupsen/logrus"

type Snippet struct {
	Name   string
	UserID UserID
	Text   string
}

func (d *DkfDB) GetUserSnippets(userID UserID) (out []Snippet, err error) {
	err = d.db.Find(&out, "user_id = ?", userID).Error
	return
}

func (d *DkfDB) CreateSnippet(userID UserID, name, text string) (out Snippet, err error) {
	out = Snippet{
		Name:   name,
		UserID: userID,
		Text:   text,
	}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) DeleteSnippet(userID UserID, name string) {
	if err := d.db.Delete(Snippet{}, "user_id = ? AND name = ?", userID, name).Error; err != nil {
		logrus.Error(err)
	}
}
