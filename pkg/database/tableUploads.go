package database

import (
	"dkforest/pkg/utils"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type UploadID int64

type Upload struct {
	ID           UploadID
	UserID       UserID
	FileName     string
	OrigFileName string
	FileSize     int64
	Password     string
	CreatedAt    time.Time
	User         User
}

// CreateUpload create file on disk in "uploads" folder, and save upload in database as well.
func CreateUpload(fileName string, content []byte, userID UserID) (*Upload, error) {
	return CreateUploadWithSize(fileName, content, userID, int64(len(content)))
}

func CreateUploadWithSize(fileName string, content []byte, userID UserID, size int64) (*Upload, error) {
	newFileName := utils.MD5([]byte(utils.GenerateToken32()))
	if err := ioutil.WriteFile(filepath.Join("uploads", newFileName), content, 0644); err != nil {
		return nil, err
	}
	upload := Upload{
		UserID:       userID,
		FileName:     newFileName,
		OrigFileName: fileName,
		FileSize:     size,
	}
	if err := DB.Create(&upload).Error; err != nil {
		logrus.Error(err)
	}
	return &upload, nil
}

func GetUploadByFileName(filename string) (out Upload, err error) {
	err = DB.First(&out, "file_name = ?", filename).Error
	return
}

func GetUploadByID(uploadID UploadID) (out Upload, err error) {
	err = DB.First(&out, "id = ?", uploadID).Error
	return
}

func GetUploads() (out []Upload, err error) {
	err = DB.Preload("User").Order("id DESC").Find(&out).Error
	return
}

func GetUserUploads(userID UserID) (out []Upload, err error) {
	err = DB.Order("id DESC").Find(&out, "user_id = ?", userID).Error
	return
}

func GetUserTotalUploadSize(userID UserID) int64 {
	var out struct{ TotalSize int64 }
	if err := DB.Raw(`SELECT SUM(file_size) as total_size FROM uploads WHERE user_id = ?`, userID).Scan(&out).Error; err != nil {
		logrus.Error(err)
	}
	return out.TotalSize
}

func DeleteOldUploads() {
	if err := DB.Exec(`DELETE FROM uploads WHERE created_at < date('now', '-1 Day')`).Error; err != nil {
		logrus.Error(err.Error())
	}
	fileInfo, err := ioutil.ReadDir("uploads")
	if err != nil {
		logrus.Error(err.Error())
		return
	}
	now := time.Now()
	for _, info := range fileInfo {
		if diff := now.Sub(info.ModTime()); diff > 24*time.Hour {
			if err := os.Remove(filepath.Join("uploads", info.Name())); err != nil {
				logrus.Error(err.Error())
			}
		}
	}
}
