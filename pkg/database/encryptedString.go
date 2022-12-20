package database

import (
	"database/sql/driver"
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	"github.com/sirupsen/logrus"
)

// EncryptedString encrypt/decrypt string value to/from the database
type EncryptedString string

// Scan EncryptedString implements scanner interface
func (s *EncryptedString) Scan(val any) error {
	v, err := utils.DecryptAES(val.([]byte), []byte(config.Global.MasterKey()))
	*s = EncryptedString(v)
	if err != nil {
		logrus.Error("Failed to Scan EncryptedString : ", err)
	}
	return err
}

// Value EncryptedString implements Valuer interface
func (s EncryptedString) Value() (driver.Value, error) {
	v, err := utils.EncryptAES([]byte(s), []byte(config.Global.MasterKey()))
	if err != nil {
		logrus.Error("Failed to Value EncryptedString : ", err)
	}
	return v, err
}

func (s EncryptedString) IsEmpty() bool {
	return s == ""
}
