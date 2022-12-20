package database

import (
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"time"
)

type Filedrop struct {
	ID           int64
	UUID         string
	FileName     string
	OrigFileName string
	FileSize     int64
	CreatedAt    time.Time
	UpdatedAt    *time.Time
}

func GetFiledropByUUID(uuid string) (out Filedrop, err error) {
	err = DB.First(&out, "uuid = ?", uuid).Error
	return
}

func GetFiledropByFileName(fileName string) (out Filedrop, err error) {
	err = DB.First(&out, "file_name = ?", fileName).Error
	return
}

func GetFiledrops() (out []Filedrop, err error) {
	err = DB.Find(&out).Error
	return
}

func CreateFiledrop() (out Filedrop, err error) {
	out.UUID = uuid.New().String()
	err = DB.Save(&out).Error
	return
}

func (d *Filedrop) DoSave() {
	if err := DB.Save(d).Error; err != nil {
		logrus.Error(err)
	}
}
