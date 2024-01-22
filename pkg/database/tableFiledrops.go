package database

import (
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	ucrypto "dkforest/pkg/utils/crypto"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

type Filedrop struct {
	ID           int64
	UUID         string
	FileName     string
	OrigFileName string
	FileSize     int64
	IV           []byte
	Password     EncryptedString
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}

func (d *DkfDB) GetFiledropByUUID(uuid string) (out Filedrop, err error) {
	err = d.db.First(&out, "uuid = ?", uuid).Error
	return
}

func (d *DkfDB) GetFiledropByFileName(fileName string) (out Filedrop, err error) {
	err = d.db.First(&out, "file_name = ?", fileName).Error
	return
}

func (d *DkfDB) GetFiledrops() (out []Filedrop, err error) {
	err = d.db.Find(&out).Error
	return
}

func (d *DkfDB) CreateFiledrop() (out Filedrop, err error) {
	out.UUID = uuid.New().String()
	out.FileName = utils.MD5([]byte(utils.GenerateToken32()))
	err = d.db.Save(&out).Error
	return
}

func (d *Filedrop) Exists() bool {
	filePath1 := filepath.Join(config.Global.ProjectFiledropPath.Get(), d.FileName)
	return utils.FileExists(filePath1)
}

func (d *Filedrop) GetContent() (*os.File, *ucrypto.StreamDecrypter, error) {
	password := []byte(d.Password)
	filePath1 := filepath.Join(config.Global.ProjectFiledropPath.Get(), d.FileName)
	f, err := os.Open(filePath1)
	if err != nil {
		return nil, nil, err
	}
	decrypter, err := utils.DecryptStream(password, d.IV, f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return f, decrypter, nil
}

func (d *Filedrop) Delete(db *DkfDB) error {
	if d.FileName != "" {
		if err := os.Remove(filepath.Join(config.Global.ProjectFiledropPath.Get(), d.FileName)); err != nil {
			logrus.Error(err)
		}
	}
	if err := db.db.Delete(&d).Error; err != nil {
		return err
	}
	return nil
}

func (d *Filedrop) DoSave(db *DkfDB) {
	if err := db.db.Save(d).Error; err != nil {
		logrus.Error(err)
	}
}
