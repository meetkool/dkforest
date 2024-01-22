package database

type ProhibitedPassword struct {
	Password string
}

func (d *DkfDB) IsPasswordProhibited(password string) bool {
	var count int64
	d.db.Table("prohibited_passwords").Where("password = ?", password).Count(&count)
	return count > 0
}
