package database

import (
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PokerTable represents a poker table in the database.
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

// GetPokerTables returns all poker tables from the database, sorted by ID and IDX.
func (d *DkfDB) GetPokerTables() ([]PokerTable, error) {
	var tables []PokerTable
	err := d.db.Order("id ASC, idx ASC").Find(&tables).Error
	return tables, err
}

// GetPokerTableBySlug returns a single poker table from the database, identified by its slug.
func (d *DkfDB) GetPokerTableBySlug(slug string) (PokerTable, error) {
	var table PokerTable
	err := d.db.First(&table, "slug = ?", slug).Error
	return table, err
}

// Piconero is the smallest unit of Monero, also known as the atomic unit.
type Piconero uint64

// ToPokerChip converts a Piconero value to a PokerChip value.
func (p Piconero) ToPokerChip() PokerChip {
	return PokerChip(p / 10_000_000)
}

// XmrStr returns a string representation of a Piconero value in Monero.
func (p Piconero) XmrStr() string {
	return fmt.Sprintf("%.12f", float64(p)/1_000_000_000_000)
}

// UsdStr returns a string representation of a Piconero value in USD.
func (p Piconero) UsdStr() string {
	return fmt.Sprintf("$%.2f", float64(p)/1_000_000_000_000*config.MoneroPrice.Load())
}

// RawString returns a raw string representation of a Piconero value.
func (p Piconero) RawString() string { return fmt.Sprintf("%d", p) }

// String returns a formatted string representation of a Piconero value.
func (p Piconero) String() string { return humanize.Comma(int64(p)) }

// PokerChip represents a chip in a poker game.
type PokerChip uint64

// ToPiconero converts a PokerChip value to a Piconero value.
func (p PokerChip) ToPiconero() Piconero {
	return Piconero(p * 10_000_000)
}

// String returns a formatted string representation of a PokerChip value.
func (p PokerChip) String() string { return humanize.Comma(int64(p)) }

// Raw returns the raw value of a PokerChip.
func (p PokerChip) Raw() uint64 { return uint64(p) }

// PokerTableAccount represents an account for a user in a poker table.
type PokerTableAccount struct {
	ID           int64
	UserID       UserID
	PokerTableID int64
	Amount       PokerChip
	AmountBet    PokerChip
}

// Save saves the PokerTableAccount to the database.
func (a *PokerTableAccount) Save(db *DkfDB) error {
	return db.db.Save(a).Error
}

// DoSave saves the PokerTableAccount to the database, logging any errors.
func (a *PokerTableAccount) DoSave(db *DkfDB) {
	if err := a.Save(db); err != nil {
		logrus.Error(err)
	}
}

// GetPositivePokerTableAccounts returns all PokerTableAccounts with a positive amount or amount_bet.
func (d *DkfDB) GetPositivePokerTableAccounts() ([]PokerTableAccount, error) {
	var accounts []PokerTableAccount
	err := d.db.Find(&accounts, "amount > 0 OR amount_bet > 0").Error
	return accounts, err
}

// GetPokerTableAccount returns a single PokerTableAccount, creating it if it doesn't exist.
func (d *DkfDB) GetPokerTableAccount(userID UserID, pokerTableID int64) (PokerTableAccount, error) {
	var account PokerTableAccount
	err := d.db.First(&account, "user_id = ? AND poker_table_id = ?", userID, pokerTableID).Error
	if err != nil {
		account = PokerTableAccount{UserID: userID, PokerTableID: pokerTableID}
		err = d.db.Create(&account).Error
	}
	return account, err
}

// GetPokerTableAccounts returns all PokerTableAccounts for a given user.
func (d *DkfDB) GetPokerTableAccounts(userID UserID) ([]PokerTableAccount, error) {
	var accounts []PokerTableAccount
	err := d.db.Find(&accounts, "user_id = ?", userID).Error
	return accounts, err
}

// GetPokerTableAccountSums returns the sum of all amounts and amount_bets in PokerTableAccounts.
func (d *
