package database

import (
	"dkforest/pkg/config"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
)

type PokerTable struct {
	ID       int64
	IDX      int64
	Slug     string
	Name     string
	MinBuyIn PokerChip
	MaxBuyIn PokerChip
	MinBet   PokerChip
	IsTest   bool
}

func (d *DkfDB) GetPokerTables() (out []PokerTable, err error) {
	err = d.db.Order("idx ASC, id").Find(&out).Error
	return
}

func (d *DkfDB) GetPokerTableBySlug(slug string) (out PokerTable, err error) {
	err = d.db.First(&out, "slug = ?", slug).Error
	return
}

// Piconero the smallest unit of Monero is 1 piconero (0.000000000001 XMR) also known as the atomic unit
// https://www.getmonero.org/resources/moneropedia/denominations.html
type Piconero uint64

func (p Piconero) ToPokerChip() PokerChip {
	return PokerChip(p / 10_000_000)
}

func (p Piconero) XmrStr() string {
	return fmt.Sprintf("%.12f", float64(p)/1_000_000_000_000)
}

func (p Piconero) UsdStr() string {
	return fmt.Sprintf("$%.2f", float64(p)/1_000_000_000_000*config.MoneroPrice.Load())
}

func (p Piconero) RawString() string { return fmt.Sprintf("%d", p) }

func (p Piconero) String() string { return humanize.Comma(int64(p)) }

type PokerChip uint64

func (p PokerChip) ToPiconero() Piconero {
	return Piconero(p * 10_000_000)
}

func (p PokerChip) String() string { return humanize.Comma(int64(p)) }

func (p PokerChip) Raw() uint64 { return uint64(p) }

type PokerTableAccount struct {
	ID           int64
	UserID       UserID
	PokerTableID int64
	Amount       PokerChip
	AmountBet    PokerChip
}

func (a *PokerTableAccount) Save(db *DkfDB) error {
	return db.db.Save(a).Error
}

func (a *PokerTableAccount) DoSave(db *DkfDB) {
	if err := a.Save(db); err != nil {
		logrus.Error(err)
	}
}

func (d *DkfDB) GetPositivePokerTableAccounts() (out []PokerTableAccount, err error) {
	err = d.db.Find(&out, "amount > 0 OR amount_bet > 0").Error
	return
}

func (d *DkfDB) GetPokerTableAccount(userID UserID, pokerTableID int64) (out PokerTableAccount, err error) {
	if err = d.db.First(&out, "user_id = ? AND poker_table_id = ?", userID, pokerTableID).Error; err != nil {
		out = PokerTableAccount{UserID: userID, PokerTableID: pokerTableID}
		err = d.db.Create(&out).Error
	}
	return
}

func (d *DkfDB) GetPokerTableAccounts(userID UserID) (out []PokerTableAccount, err error) {
	err = d.db.Find(&out, "user_id = ?", userID).Error
	return
}

func (d *DkfDB) GetPokerTableAccountSums() (sumAmounts, sumBets PokerChip, err error) {
	var tmp struct{ SumAmounts, SumBets PokerChip }
	err = d.db.Raw(`SELECT SUM(amount) AS sum_amounts, SUM(amount_bet) AS sum_bets FROM poker_table_accounts INNER JOIN poker_tables t ON t.id= poker_table_id WHERE t.is_test = 0`).Scan(&tmp).Error
	return tmp.SumAmounts, tmp.SumBets, err
}

func (d *DkfDB) PokerTableAccountBet(userID UserID, pokerTableID int64, bet PokerChip) (err error) {
	err = d.db.Exec(`UPDATE poker_table_accounts SET amount = amount - ?, amount_bet = amount_bet + ? WHERE user_id = ? AND poker_table_id = ?`,
		bet, bet, userID, pokerTableID).Error
	return
}

func (d *DkfDB) PokerTableAccountRefundPartialBet(userID UserID, pokerTableID int64, diff PokerChip) (err error) {
	err = d.db.Exec(`UPDATE poker_table_accounts SET amount = amount + ?, amount_bet = amount_bet - ? WHERE user_id = ? AND poker_table_id = ?`,
		diff, diff, userID, pokerTableID).Error
	return
}

func (d *DkfDB) PokerTableAccountGain(userID UserID, pokerTableID int64, gain PokerChip) (err error) {
	err = d.db.Exec(`UPDATE poker_table_accounts SET amount = amount + ?, amount_bet = 0 WHERE user_id = ? AND poker_table_id = ?`,
		gain, userID, pokerTableID).Error
	return
}
