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

func CreateUserBadge(userID UserID, badgeID int64) error {
	ub := UserBadge{UserID: userID, BadgeID: badgeID}
	return DB.Create(&ub).Error
}

func GetUsersBadges() (out []UserBadge, err error) {
	err = DB.Preload("User").Preload("Badge").Order("created_at").Find(&out).Error
	return
}
