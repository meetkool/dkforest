package handlers

import (
	"bytes"
	cryptoRand "crypto/rand"
	"crypto/sha256"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	"dkforest/pkg/utils"
	"dkforest/pkg/utils/crypto"
	hutils "dkforest/pkg/web/handlers/utils"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/asaskevich/govalidator"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func FileDropHandler(c echo.Context) error {
	const filedropTmplName = "standalone.filedrop"
	uuidParam := c.Param("uuid")
	db := c.Get("database").(*database.DkfDB)
	//if c.Request().ContentLength > config.MaxUserFileUploadSize {
	//	data.Error = fmt.Sprintf("The maximum file size is %s", humanize.Bytes(config.MaxUserFileUploadSize))
	//	return c.Render(http.StatusOK, "chat-top-bar", data)
	//}

	filedrop, err := db.GetFiledropByUUID(uuidParam)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	if filedrop.FileSize > 0 {
		return c.Redirect(http.StatusFound, "/")
	}

	var data fileDropData
	data.Filedrop = filedrop

	if c.Request().Method == http.MethodGet {
		return c.Render(http.StatusOK, filedropTmplName, data)
	}

	file, handler, uploadErr := c.Request().FormFile("file")
	if uploadErr != nil {
		data.Error = uploadErr.Error()
		return c.Render(http.StatusOK, filedropTmplName, data)
	}

	defer file.Close()
	origFileName := handler.Filename
	//if handler.Size > config.MaxUserFileUploadSize {
	//	return nil, html, fmt.Errorf("the maximum file size is %s", humanize.Bytes(config.MaxUserFileUploadSize))
	//}
	if !govalidator.StringLength(origFileName, "3", "50") {
		data.Error = "invalid file name, 3-50 characters"
		return c.Render(http.StatusOK, filedropTmplName, data)
	}

	password := make([]byte, 16)
	_, _ = cryptoRand.Read(password)

	encrypter, err := utils.EncryptStream(password, file)
	if err != nil {
		data.Error = err.Error()
		return c.Render(http.StatusOK, filedropTmplName, data)
	}

	outFile, err := os.OpenFile(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedrop.FileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		data.Error = err.Error()
		return c.Render(http.StatusOK, filedropTmplName, data)
	}
	defer outFile.Close()
	written, err := io.Copy(outFile, encrypter)
	if err != nil {
		data.Error = err.Error()
		return c.Render(http.StatusOK, filedropTmplName, data)
	}

	filedrop.Password = database.EncryptedString(password)
	filedrop.IV = encrypter.Meta().IV
	filedrop.OrigFileName = origFileName
	filedrop.FileSize = written
	filedrop.DoSave(db)

	data.Success = "File uploaded successfully"
	return c.Render(http.StatusOK, filedropTmplName, data)
}

func FileDropDkfUploadHandler(c echo.Context) error {
	db := c.Get("database").(*database.DkfDB)
	// Init
	if c.Request().PostFormValue("init") != "" {
		filedropUUID := c.Param("uuid")
		_, err := db.GetFiledropByUUID(filedropUUID)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}

		_ = os.Mkdir(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID), 0755)
		metadataPath := filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID, "metadata")

		fileName := c.Request().PostFormValue("fileName")
		fileSize := c.Request().PostFormValue("fileSize")
		fileSha256 := c.Request().PostFormValue("fileSha256")
		chunkSize := c.Request().PostFormValue("chunkSize")
		nbChunks := c.Request().PostFormValue("nbChunks")
		data := []byte(fileName + "\n" + fileSize + "\n" + fileSha256 + "\n" + chunkSize + "\n" + nbChunks + "\n")

		if _, err := os.Stat(metadataPath); err != nil {
			if err := os.WriteFile(metadataPath, data, 0644); err != nil {
				logrus.Error(err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
		} else {
			by, err := os.ReadFile(metadataPath)
			if err != nil {
				logrus.Error(err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if bytes.Compare(by, data) != 0 {
				err := errors.New("metadata file already exists with different configuration")
				logrus.Error(err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
		}

		return c.NoContent(http.StatusOK)
	}

	// completed
	if c.Request().PostFormValue("completed") != "" {
		filedropUUID := c.Param("uuid")

		filedrop, err := db.GetFiledropByUUID(filedropUUID)
		if err != nil {
			return c.Redirect(http.StatusFound, "/")
		}

		dirEntries, _ := os.ReadDir(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID))
		fileNames := make([]string, 0)
		for _, dirEntry := range dirEntries {
			if !strings.HasPrefix(dirEntry.Name(), "part_") {
				continue
			}
			fileNames = append(fileNames, dirEntry.Name())
		}
		sort.Slice(fileNames, func(i, j int) bool {
			a := strings.Split(fileNames[i], "_")[1]
			b := strings.Split(fileNames[j], "_")[1]
			numA, _ := strconv.Atoi(a)
			numB, _ := strconv.Atoi(b)
			return numA < numB
		})

		metadata, err := os.ReadFile(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID, "metadata"))
		if err != nil {
			logrus.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		lines := strings.Split(string(metadata), "\n")
		origFileName := lines[0]
		fileSha256 := lines[2]

		f, err := os.OpenFile(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedrop.FileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			logrus.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		defer f.Close()
		h := sha256.New()

		password := make([]byte, 16)
		_, _ = cryptoRand.Read(password)

		stream, _, iv, err := crypto.NewCtrStram(password)
		if err != nil {
			logrus.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
		written := int64(0)
		for _, fileName := range fileNames {
			by, err := os.ReadFile(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID, fileName))
			if err != nil {
				logrus.Error(err)
				return c.NoContent(http.StatusInternalServerError)
			}

			dst := make([]byte, len(by))
			_, _ = h.Write(by)
			stream.XORKeyStream(dst, by)
			_, err = f.Write(dst)
			if err != nil {
				logrus.Error(err)
				return c.NoContent(http.StatusInternalServerError)
			}
			written += int64(len(by))
		}

		newFileSha256 := hex.EncodeToString(h.Sum(nil))

		if newFileSha256 != fileSha256 {
			logrus.Errorf("%s != %s", newFileSha256, fileSha256)
			return c.NoContent(http.StatusInternalServerError)
		}

		// Cleanup
		_ = os.RemoveAll(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID))

		filedrop.Password = database.EncryptedString(password)
		filedrop.IV = iv
		filedrop.OrigFileName = origFileName
		filedrop.FileSize = written
		filedrop.DoSave(db)

		return c.NoContent(http.StatusOK)
	}

	filedropUUID := c.Param("uuid")

	{
		chunkFileName := c.Request().PostFormValue("chunkFileName")
		if chunkFileName != "" {
			if _, err := os.Stat(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID, chunkFileName)); err != nil {
				return c.NoContent(http.StatusOK)
			}
			// Let's use the teapot response (because why not) to say that we already have the file
			return c.NoContent(http.StatusTeapot)
		}
	}

	_, err := db.GetFiledropByUUID(filedropUUID)
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	file, handler, err := c.Request().FormFile("file")
	if err != nil {
		logrus.Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer file.Close()
	fileName := handler.Filename
	by, _ := io.ReadAll(file)
	p := filepath.Join(config.Global.ProjectFiledropPath.Get(), filedropUUID, fileName)
	if err := os.WriteFile(p, by, 0644); err != nil {
		logrus.Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

func FileDropDkfDownloadHandler(c echo.Context) error {
	filedropUUID := c.Param("uuid")
	db := c.Get("database").(*database.DkfDB)
	filedrop, err := db.GetFiledropByUUID(filedropUUID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	maxChunkSize := int64(2 << 20) // 2MB
	f, err := os.Open(filepath.Join(config.Global.ProjectFiledropPath.Get(), filedrop.FileName))
	if err != nil {
		logrus.Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	defer f.Close()

	init := c.Request().PostFormValue("init")
	if init != "" {
		fs, err := f.Stat()
		if err != nil {
			logrus.Error(err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}

		// Calculate sha256 of file
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			logrus.Error(err.Error())
			return c.NoContent(http.StatusInternalServerError)
		}
		fileSha256 := hex.EncodeToString(h.Sum(nil))

		fileSize := fs.Size()
		nbChunks := int64(math.Ceil(float64(fileSize) / float64(maxChunkSize)))
		b64Password := base64.StdEncoding.EncodeToString([]byte(filedrop.Password))
		b64IV := base64.StdEncoding.EncodeToString(filedrop.IV)
		body := fmt.Sprintf("%s\n%s\n%s\n%s\n%d\n%d\n", filedrop.OrigFileName, b64Password, b64IV, fileSha256, fileSize, nbChunks)
		return c.String(http.StatusOK, body)
	}

	chunkNum := utils.DoParseInt64(c.Request().PostFormValue("chunk"))

	buf := make([]byte, maxChunkSize)
	n, err := f.ReadAt(buf, chunkNum*maxChunkSize)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			logrus.Error(err)
			return c.NoContent(http.StatusInternalServerError)
		}
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=chunk_%d", chunkNum))
	if _, err := io.Copy(c.Response().Writer, bytes.NewReader(buf[:n])); err != nil {
		logrus.Error(err)
		return c.NoContent(http.StatusInternalServerError)
	}
	c.Response().Flush()
	return nil
}

func FileDropDownloadHandler(c echo.Context) error {
	authUser, ok := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	if !ok {
		return c.Redirect(http.StatusFound, "/")
	}

	fileName := c.Param("fileName")
	if !utils.FileExists(filepath.Join(config.Global.ProjectDownloadsPath.Get(), fileName)) {
		logrus.Error(fileName + " does not exists")
		return c.Redirect(http.StatusFound, "/")
	}

	userNbDownloaded := db.UserNbDownloaded(authUser.ID, fileName)

	// Display captcha to new users, or old users if they already downloaded the file.
	if !authUser.AccountOldEnough() || userNbDownloaded >= 1 {
		// Captcha for bigger files
		var data captchaRequiredData
		data.CaptchaDescription = "Captcha required"
		if !authUser.AccountOldEnough() {
			data.CaptchaDescription = fmt.Sprintf("Account that are less than 3 days old must complete the captcha to download files bigger than %s", humanize.Bytes(config.MaxFileSizeBeforeDownload))
		} else if userNbDownloaded >= 1 {
			data.CaptchaDescription = fmt.Sprintf("For the second download onward of a file bigger than %s, you must complete the captcha", humanize.Bytes(config.MaxFileSizeBeforeDownload))
		}
		data.CaptchaID, data.CaptchaImg = captcha.New()
		const captchaRequiredTmpl = "captcha-required"
		if c.Request().Method == http.MethodGet {
			return c.Render(http.StatusOK, captchaRequiredTmpl, data)
		}
		captchaID := c.Request().PostFormValue("captcha_id")
		captchaInput := c.Request().PostFormValue("captcha")
		if err := hutils.CaptchaVerifyString(c, captchaID, captchaInput); err != nil {
			data.ErrCaptcha = err.Error()
			return c.Render(http.StatusOK, captchaRequiredTmpl, data)
		}
	}

	// Keep track of user downloads
	if _, err := db.CreateDownload(authUser.ID, fileName); err != nil {
		logrus.Error(err)
	}

	f, err := os.Open(filepath.Join(config.Global.ProjectDownloadsPath.Get(), fileName))
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}
	fi, err := f.Stat()
	if err != nil {
		return c.Redirect(http.StatusFound, "/")
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", fileName))
	http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), f)
	return nil
}
