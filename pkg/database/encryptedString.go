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
	var err error
	*s, err = decryptAESMaster(val.([]byte))
	if err != nil {
		logrus.Error("Failed to Scan EncryptedString : ", err)
	}
	return err
}

// Value EncryptedString implements Valuer interface
func (s EncryptedString) Value() (driver.Value, error) {
	v, err := encryptAESMaster([]byte(s))
	if err != nil {
		logrus.Error("Failed to Value EncryptedString : ", err)
	}
	return v, err
}

func (s EncryptedString) IsEmpty() bool {
	return s == ""
}

// decryptAESMaster is a helper function to decrypt the value using AES
func decryptAESMaster(value []byte) (EncryptedString, error) {
	return EncryptedString(utils.DecryptAESMaster(value)), nil
}

// encryptAESMaster is a helper function to encrypt the value using AES
func encryptAESMaster(value []byte) ([]byte, error) {
	return utils.EncryptAESMaster(value)
}
