package database

import "time"

type OnionBlacklist struct {
	Md5       string
	CreatedAt time.Time
}

func GetOnionBlacklist(hash string) (out OnionBlacklist, err error) {
	err = DB.First(&out, "md5 = ?", hash).Error
	return
}
