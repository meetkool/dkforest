package v1

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	hutils "dkforest/pkg/web/handlers/utils"
	"errors"
	"github.com/asaskevich/govalidator"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"mime/multipart"
)

type UploadInterceptor struct{}

func (i UploadInterceptor) InterceptMsg(cmd *Command) {
	if file, handler, uploadErr := cmd.c.Request().FormFile("file"); uploadErr == nil {
		// Save file on disk & database & append file link to html
		var err error
		cmd.upload, err = handleUploadedFile(file, handler, cmd.authUser)
		if err != nil {
			cmd.err = err
			return
		}
	}
}

func handleUploadedFile(file multipart.File, handler *multipart.FileHeader, authUser *database.User) (*database.Upload, error) {
	defer file.Close()
	if !authUser.CanUpload() {
		return nil, hutils.AccountTooYoungErr
	}
	userSizeUploaded := database.GetUserTotalUploadSize(authUser.ID)
	if handler.Size+userSizeUploaded > 100<<20 {
		return nil, errors.New("user upload limit reached (100 MB)")
	}
	origFileName := handler.Filename
	if handler.Size > 30<<20 {
		return nil, errors.New("the maximum file size is 30 MB")
	}
	if !govalidator.StringLength(origFileName, "3", "50") {
		return nil, errors.New("invalid file name, 3-50 characters")
	}
	origFileName = tzRgx.ReplaceAllString(origFileName, "xxxx-xx-xx at xx.xx.xx XX")
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Validate image type and determine extension
	mimeType := handler.Header.Get("Content-Type")
	if mimeType == "image/jpeg" {
		fileBytes, err = utils.ReencodeJpg(fileBytes)
	} else if mimeType == "image/png" {
		fileBytes, err = utils.ReencodePng(fileBytes)
	}
	if err != nil {
		return nil, err
	}

	// Uploaded files are encrypted on disk
	fileBytes, _ = utils.EncryptAES(fileBytes, []byte(config.Global.MasterKey()))

	upload, err := database.CreateUploadWithSize(origFileName, fileBytes, authUser.ID, handler.Size)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return upload, nil
}
