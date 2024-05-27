package database

import (
	"dkforest/pkg/config"
	"fmt"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	wallet1 "github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/sirupsen/logrus"
	"image"
	"time"
)

type XmrInvoice struct {
	ID              int64
	UserID          UserID
	ProductID       int64
	Address         string
	AmountRequested int64
	AmountReceived  *int64
	Confirmations   int64
	CreatedAt       time.Time
}

func (d *DkfDB) CreateXmrInvoice(userID UserID, productID int64) (out XmrInvoice, err error) {
	err = d.db.Where("user_id = ? AND product_id = ? AND amount_received IS NULL", userID, productID).First(&out).Error
	if err == nil {
		return
	}
	resp, err := config.Xmr().CreateAddress(&wallet1.RequestCreateAddress{})
	if err != nil {
		logrus.Error(err)
		return
	}
	out = XmrInvoice{
		UserID:          userID,
		ProductID:       productID,
		Address:         resp.Address,
		AmountRequested: 10,
	}
	if err = d.db.Create(&out).Error; err != nil {
		return
	}
	return
}

func (d *DkfDB) GetXmrInvoiceByAddress(address string) (out XmrInvoice, err error) {
	err = d.db.Where("address = ?", address).First(&out).Error
	return
}

// GetURL monero:599sdD99LUJ8a7ubSetcRS58zRBQ9uqcMGJED5eg1J9rd8Ktq1A93zfi9iMsKbKVPq3XvbTwKLmZ48mcWe9Kwc5AUKdbyV8?tx_amount=0.010000000000&recipient_name=n0tr1v&tx_description=desc
func (i XmrInvoice) GetURL() string {
	description := "membership"
	recipient := "n0tr1v"
	amount := 0.01
	return fmt.Sprintf("monero:%s?tx_amount=%f&recipient_name=%s&tx_description=%s", i.Address, amount, recipient, description)
}

func (i XmrInvoice) GetImage() (image.Image, error) {
	b, err := qr.Encode(i.GetURL(), qr.L, qr.Auto)
	if err != nil {
		return nil, err
	}
	b, err = barcode.Scale(b, 150, 150)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (i *XmrInvoice) DoSave(db *DkfDB) {
	if err := db.db.Save(i).Error; err != nil {
		logrus.Error(err)
	}
}

type Product struct {
	ID          int64
	Name        string
	Description string
	Price       int64
	CreatedAt   time.Time
}
