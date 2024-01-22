package database

import (
	"dkforest/pkg/config"
	"dkforest/pkg/utils"
	html2 "html"
	"io"
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

func (u *Upload) GetHTMLLink() string {
	escapedOrigFileName := html2.EscapeString(u.OrigFileName)
	return `<a href="/uploads/` + u.FileName + `" rel="noopener noreferrer" target="_blank">` + escapedOrigFileName + `</a>`
}

func (u *Upload) GetContent() (os.FileInfo, []byte, error) {
	filePath1 := filepath.Join(config.Global.ProjectUploadsPath.Get(), u.FileName)
	f, err := os.Open(filePath1)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	fileBytes, _ := io.ReadAll(f)
	decFileBytes, err := utils.DecryptAESMaster(fileBytes)
	if err != nil {
		decFileBytes = fileBytes
	}
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	return fi, decFileBytes, nil
}

func (u *Upload) Exists() bool {
	filePath1 := filepath.Join(config.Global.ProjectUploadsPath.Get(), u.FileName)
	return utils.FileExists(filePath1)
}

func (u *Upload) Delete(db *DkfDB) error {
	if err := os.Remove(filepath.Join(config.Global.ProjectUploadsPath.Get(), u.FileName)); err != nil {
		return err
	}
	if err := db.db.Delete(&u).Error; err != nil {
		return err
	}
	return nil
}

// CreateUpload create file on disk in "uploads" folder, and save upload in database as well.
func (d *DkfDB) CreateUpload(fileName string, content []byte, userID UserID) (*Upload, error) {
	return d.createUploadWithSize(fileName, content, userID, int64(len(content)))
}

func (d *DkfDB) CreateEncryptedUploadWithSize(fileName string, content []byte, userID UserID, size int64) (*Upload, error) {
	encryptedContent, err := utils.EncryptAESMaster(content)
	if err != nil {
		return nil, err
	}
	return d.createUploadWithSize(fileName, encryptedContent, userID, size)
}

func (d *DkfDB) createUploadWithSize(fileName string, content []byte, userID UserID, size int64) (*Upload, error) {
	newFileName := utils.MD5([]byte(utils.GenerateToken32()))
	if err := os.WriteFile(filepath.Join(config.Global.ProjectUploadsPath.Get(), newFileName), content, 0644); err != nil {
		return nil, err
	}
	upload := Upload{
		UserID:       userID,
		FileName:     newFileName,
		OrigFileName: fileName,
		FileSize:     size,
	}
	if err := d.db.Create(&upload).Error; err != nil {
		logrus.Error(err)
	}
	return &upload, nil
}

func (d *DkfDB) GetUploadByFileName(filename string) (out Upload, err error) {
	err = d.db.First(&out, "file_name = ?", filename).Error
	return
}

func (d *DkfDB) GetUploadByID(uploadID UploadID) (out Upload, err error) {
	err = d.db.First(&out, "id = ?", uploadID).Error
	return
}

func (d *DkfDB) GetUploads() (out []Upload, err error) {
	err = d.db.Preload("User").Order("id DESC").Find(&out).Error
	return
}

func (d *DkfDB) GetUserUploads(userID UserID) (out []Upload, err error) {
	err = d.db.Order("id DESC").Find(&out, "user_id = ?", userID).Error
	return
}

func (d *DkfDB) GetUserTotalUploadSize(userID UserID) int64 {
	var out struct{ TotalSize int64 }
	if err := d.db.Raw(`SELECT SUM(file_size) as total_size FROM uploads WHERE user_id = ?`, userID).Scan(&out).Error; err != nil {
		logrus.Error(err)
	}
	return out.TotalSize
}

func (d *DkfDB) DeleteOldUploads() {
	if err := d.db.Exec(`DELETE FROM uploads WHERE created_at < date('now', '-1 Day')`).Error; err != nil {
		logrus.Error(err.Error())
	}
	entries, err := os.ReadDir(config.Global.ProjectUploadsPath.Get())
	if err != nil {
		logrus.Error(err.Error())
		return
	}
	now := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if diff := now.Sub(info.ModTime()); diff > 24*time.Hour {
			if err := os.Remove(filepath.Join(config.Global.ProjectUploadsPath.Get(), info.Name())); err != nil {
				logrus.Error(err.Error())
			}
		}
	}
}
