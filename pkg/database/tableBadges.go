package database

import "time"

type Badge struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type UserBadge struct {
	UserID    UserID
	BadgeID   int64
	CreatedAt time.Time
	User      User
	Badge     Badge
}

func (d *DkfDB) CreateUserBadge(userID UserID, badgeID int64) error {
	ub := UserBadge{UserID: userID, BadgeID: badgeID}
	return d.db.Create(&ub).Error
}

func (d *DkfDB) GetUsersBadges() (out []UserBadge, err error) {
	err = d.db.Preload("User").Preload("Badge").Order("created_at").Find(&out).Error
	return
}
