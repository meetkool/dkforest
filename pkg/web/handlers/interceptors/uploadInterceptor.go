package interceptors

import (
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"dkforest/pkg/web/handlers/interceptors/command"
	hutils "dkforest/pkg/web/handlers/utils"
	"errors"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"io"
	"mime/multipart"
)

type UploadInterceptor struct{}

func (i UploadInterceptor) InterceptMsg(cmd *command.Command) {
	if file, handler, uploadErr := cmd.C.Request().FormFile("file"); uploadErr == nil {
		// Save file on disk & database & append file link to html
		var err error
		cmd.Upload, err = handleUploadedFile(cmd.DB, file, handler, cmd.AuthUser)
		if err != nil {
			cmd.Err = err
			return
		}
	}
}

func handleUploadedFile(db *database.DkfDB, file multipart.File, handler *multipart.FileHeader, authUser *database.User) (*database.Upload, error) {
	defer file.Close()
	if !authUser.CanUpload() {
		return nil, hutils.AccountTooYoungErr
	}
	userSizeUploaded := db.GetUserTotalUploadSize(authUser.ID)
	if handler.Size+userSizeUploaded > config.MaxUserTotalUploadSize {
		return nil, fmt.Errorf("user upload limit reached (%s)", humanize.Bytes(config.MaxUserTotalUploadSize))
	}
	origFileName := handler.Filename
	if handler.Size > config.MaxUserFileUploadSize {
		return nil, fmt.Errorf("the maximum file size is %s", humanize.Bytes(config.MaxUserFileUploadSize))
	}
	if !govalidator.StringLength(origFileName, "3", "50") {
		return nil, errors.New("invalid file name, 3-50 characters")
	}
	if !govalidator.IsPrintableASCII(origFileName) {
		return nil, errors.New("file name must be ascii printable only")
	}
	origFileName = tzRgx.ReplaceAllString(origFileName, "xxxx-xx-xx at xx.xx.xx XX")
	origFileName = tz1Rgx.ReplaceAllString(origFileName, "xxxx-xx-xx xx-xx-xx")
	origFileName = tz3Rgx.ReplaceAllString(origFileName, "xxxx-xx-xx xxxxxx")
	origFileName = tz4Rgx.ReplaceAllString(origFileName, "xxxx-xx-xx_xx_xx_xx")
	fileBytes, err := io.ReadAll(file)
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
	upload, err := db.CreateEncryptedUploadWithSize(origFileName, fileBytes, authUser.ID, handler.Size)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return upload, nil
}
