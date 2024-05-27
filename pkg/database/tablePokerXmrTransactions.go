package database

import (
	"dkforest/pkg/utils"
	"github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/sirupsen/logrus"
	"time"
)

const (
	PokerXmrTransactionStatusPending = 1
	PokerXmrTransactionStatusSuccess = 2
	PokerXmrTransactionStatusFailed  = 3
)

type PokerXmrTransaction struct {
	ID            int64
	Status        int64
	TxID          string
	UserID        UserID
	Address       string
	Amount        Piconero
	Fee           Piconero
	Confirmations uint64
	Height        uint64
	IsIn          bool
	Processed     bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	User          User
}

func (t *PokerXmrTransaction) SetStatus(db *DkfDB, newStatus int64) error {
	return db.db.Model(t).Select("Status").Updates(PokerXmrTransaction{Status: newStatus}).Error
}

func (t *PokerXmrTransaction) HasEnoughConfirmations() bool {
	return t.Confirmations >= t.ConfirmationsNeeded()
}

func (t *PokerXmrTransaction) ConfirmationsNeeded() uint64 {
	return utils.Ternary(t.Amount.ToPokerChip() <= 20000, uint64(2), 10)
}

func (d *DkfDB) GetPokerXmrTransactionsSumIn() (out Piconero, err error) {
	var tmp struct{ Amount Piconero }
	err = d.db.Raw(`SELECT SUM(amount) AS amount FROM poker_xmr_transactions WHERE is_in = 1 AND confirmations >= 10`).Scan(&tmp).Error
	return tmp.Amount, err
}

func (d *DkfDB) GetPokerXmrTransactionsSumOut() (out Piconero, err error) {
	var tmp struct {
		Amount Piconero
		Fee    Piconero
	}
	err = d.db.Raw(`SELECT SUM(amount) AS amount, SUM(fee) AS fee FROM poker_xmr_transactions WHERE is_in = 0 AND status = 2`).Scan(&tmp).Error
	return tmp.Amount + tmp.Fee, err
}

func (d *DkfDB) GetPokerXmrPendingTransactions() (out []PokerXmrTransaction, err error) {
	err = d.db.Find(&out, "is_in = 0 AND status = 1").Error
	return
}

func (d *DkfDB) GetLastUserWithdrawPokerXmrTransaction(userID UserID) (out PokerXmrTransaction, err error) {
	err = d.db.Order("id DESC").First(&out, "user_id = ? AND is_in = 0", userID).Error
	return
}

func (d *DkfDB) GetUserPokerXmrTransactions(userID UserID) (out []PokerXmrTransaction, err error) {
	err = d.db.Order("id DESC").Find(&out, "user_id = ?", userID).Error
	return
}

func (d *DkfDB) GetPokerXmrTransaction(txID string) (out PokerXmrTransaction, err error) {
	err = d.db.First(&out, "tx_id = ?", txID).Error
	return
}

func (d *DkfDB) CreatePokerXmrInTransaction(userID UserID, transfer *wallet.Transfer) (out PokerXmrTransaction, err error) {
	out = PokerXmrTransaction{
		TxID:          transfer.TxID,
		UserID:        userID,
		Address:       transfer.Address,
		Height:        transfer.Height,
		Amount:        Piconero(transfer.Amount),
		Fee:           Piconero(transfer.Fee),
		Confirmations: transfer.Confirmations,
		IsIn:          true,
	}
	err = d.db.Create(&out).Error
	return
}

func (d *DkfDB) CreatePokerXmrTransaction(userID UserID, transfer *wallet.ResponseTransfer) (out PokerXmrTransaction, err error) {
	out = PokerXmrTransaction{
		Status: PokerXmrTransactionStatusPending,
		UserID: userID,
		Amount: Piconero(transfer.Amount),
		Fee:    Piconero(transfer.Fee),
		IsIn:   false}
	err = d.db.Create(&out).Error
	return
}

func (t *PokerXmrTransaction) Save(db *DkfDB) error {
	return db.db.Save(t).Error
}

func (t *PokerXmrTransaction) DoSave(db *DkfDB) {
	if err := t.Save(db); err != nil {
		logrus.Error(err)
	}
}
