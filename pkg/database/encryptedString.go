package database

import (
	"database/sql/driver"
	"dkforest/pkg/utils"
	"github.com/sirupsen/logrus"
)

// EncryptedString encrypt/decrypt string value to/from the database
type EncryptedString string

// Scan EncryptedString implements scanner interface
func (s *EncryptedString) Scan(val any) error {
	v, err := utils.DecryptAESMaster(val.([]byte))
	*s = EncryptedString(v)
	if err != nil {
		logrus.Error("Failed to Scan EncryptedString : ", err)
	}
	return err
}

// Value EncryptedString implements Valuer interface
func (s EncryptedString) Value() (driver.Value, error) {
	v, err := utils.EncryptAESMaster([]byte(s))
	if err != nil {
		logrus.Error("Failed to Value EncryptedString : ", err)
	}
	return v, err
}

func (s EncryptedString) IsEmpty() bool {
	return s == ""
}
