package config

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

func Load() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func Get(key string) string {
	return os.Getenv(key)
}

package context

import (
	"context"
	"time"
)

func Background() context.Context {
	return context.Background()
}

func WithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

package database

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/sirupsen/logrus"
)

type DkfDB struct {
	db *gorm.DB
}

func New(cfg Config) (*DkfDB, error) {
	db, err := gorm.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName))
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&XmrInvoice{})
	return &DkfDB{db: db}, nil
}

func (d *DkfDB) Close() error {
	return d.db.Close()
}

func (d *DkfDB) CreateXmrInvoice(userID UserID, productID int64) (out XmrInvoice, err error) {
	err = d.db.Where("user_id = ? AND product_id = ? AND amount_received IS NULL", userID, productID).First(&out).Error
	if err == nil {
		return
	}
	resp, err := wallet.CreateAddress(&wallet.RequestCreateAddress{})
	if err != nil {
		logrus.Error(err)
		return
