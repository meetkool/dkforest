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

type MemeID int64

type Meme struct {
	ID           MemeID
	Slug         string
	FileName     string
	OrigFileName string
	FileSize     int64
	CreatedAt    time.Time
}

func (u *Meme) DoSave(db *DkfDB) {
	if err := db.db.Save(u).Error; err != nil {
		logrus.Error(err)
	}
}

func (u *Meme) GetHTMLLink() string {
	escapedOrigFileName := html2.EscapeString(u.OrigFileName)
	return `<a href="/memes/` + u.FileName + `" rel="noopener noreferrer" target="_blank">` + escapedOrigFileName + `</a>`
}

func (u *Meme) GetContent() (os.FileInfo, []byte, error) {
	filePath1 := filepath.Join(config.Global.ProjectMemesPath.Get(), u.FileName)
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

func (u *Meme) Exists() bool {
	filePath1 := filepath.Join(config.Global.ProjectMemesPath.Get(), u.FileName)
	return utils.FileExists(filePath1)
}

func (u *Meme) Delete(db *DkfDB) error {
	if err := os.Remove(filepath.Join(config.Global.ProjectMemesPath.Get(), u.FileName)); err != nil {
		return err
	}
	if err := db.db.Delete(&u).Error; err != nil {
		return err
	}
	return nil
}

// CreateMeme create file on disk in "memes" folder, and save meme in database as well.
func (d *DkfDB) CreateMeme(fileName string, content []byte) (*Meme, error) {
	return d.createMemeWithSize(fileName, content, int64(len(content)))
}

func (d *DkfDB) CreateEncryptedMemeWithSize(fileName string, content []byte, size int64) (*Meme, error) {
	encryptedContent, err := utils.EncryptAESMaster(content)
	if err != nil {
		return nil, err
	}
	return d.createMemeWithSize(fileName, encryptedContent, size)
}

func (d *DkfDB) createMemeWithSize(fileName string, content []byte, size int64) (*Meme, error) {
	newFileName := utils.MD5([]byte(utils.GenerateToken32()))
	if err := os.WriteFile(filepath.Join(config.Global.ProjectMemesPath.Get(), newFileName), content, 0644); err != nil {
		return nil, err
	}
	meme := Meme{
		FileName:     newFileName,
		OrigFileName: fileName,
		FileSize:     size,
	}
	if err := d.db.Create(&meme).Error; err != nil {
		logrus.Error(err)
	}
	return &meme, nil
}

func (d *DkfDB) GetMemeByFileName(filename string) (out Meme, err error) {
	err = d.db.First(&out, "file_name = ?", filename).Error
	return
}

func (d *DkfDB) GetMemeBySlug(slug string) (out Meme, err error) {
	err = d.db.First(&out, "slug = ?", slug).Error
	return
}

func (d *DkfDB) GetMemeByID(memeID MemeID) (out Meme, err error) {
	err = d.db.First(&out, "id = ?", memeID).Error
	return
}

func (d *DkfDB) GetMemes() (out []Meme, err error) {
	err = d.db.Order("id DESC").Find(&out).Error
	return
}
