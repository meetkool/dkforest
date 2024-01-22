package database

import "time"

type SpamFilter struct {
	ID        int64
	Action    int64
	Filter    string
	IsRegex   bool
	CreatedAt time.Time
}

func (d *DkfDB) GetSpamFilters() (out []SpamFilter, err error) {
	err = d.db.Find(&out).Error
	return
}

func (d *DkfDB) CreateOrEditSpamFilter(id int64, filter string, isRegex bool, action int64) (out SpamFilter, err error) {
	out.ID = id
	out.Filter = filter
	out.IsRegex = isRegex
	out.Action = action
	err = d.db.Save(&out).Error
	return
}

func (d *DkfDB) DeleteSpamFilterByID(id int64) error {
	return d.db.Delete(SpamFilter{}, "id = ?", id).Error
}
