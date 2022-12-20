package database

import "github.com/sirupsen/logrus"

type Snippet struct {
	Name   string
	UserID UserID
	Text   string
}

func GetUserSnippets(userID UserID) (out []Snippet, err error) {
	err = DB.Find(&out, "user_id = ?", userID).Error
	return
}

func CreateSnippet(userID UserID, name, text string) (out Snippet, err error) {
	out = Snippet{
		Name:   name,
		UserID: userID,
		Text:   text,
	}
	err = DB.Create(&out).Error
	return
}

func DeleteSnippet(userID UserID, name string) {
	if err := DB.Delete(Snippet{}, "user_id = ? AND name = ?", userID, name).Error; err != nil {
		logrus.Error(err)
	}
}
