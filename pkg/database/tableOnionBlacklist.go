package database

import "time"

type OnionBlacklist struct {
	Md5       string
	CreatedAt time.Time
}

func (d *DkfDB) GetOnionBlacklist(hash string) (out OnionBlacklist, err error) {
	err = d.db.First(&out, "md5 = ?", hash).Error
	return
}
