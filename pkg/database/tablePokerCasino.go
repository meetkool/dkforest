package database

import "fmt"

// PokerCasino table, should always be one row
type PokerCasino struct {
	ID            int64
	Rake          PokerChip
	HandsPlayed   int64
	TotalRakeBack int64
}

func (PokerCasino) TableName() string { return "poker_casino" }

func (d *DkfDB) GetPokerCasino() (out PokerCasino) {
	if err := d.db.Model(PokerCasino{}).First(&out).Error; err != nil {
		if err = d.db.Create(&out).Error; err != nil {
			fmt.Println(err)
		}
	}
	return
}

func (d *DkfDB) IncrPokerCasinoRake(rake, rakeBack PokerChip) (err error) {
	err = d.db.Exec(`UPDATE poker_casino SET rake = rake + ?, total_rake_back = total_rake_back + ?, hands_played = hands_played + 1 WHERE id = 1`, rake, rakeBack).Error
	return
}
