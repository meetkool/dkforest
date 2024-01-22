package handlers

import (
	"bytes"
	"dkforest/pkg/captcha"
	"dkforest/pkg/config"
	"dkforest/pkg/database"
	hutils "dkforest/pkg/web/handlers/utils"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/labstack/echo"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"unicode/utf8"
)

func isImageMimeType(mimeType string) bool {
	return mimeType == "image/jpeg" ||
		mimeType == "image/png" ||
		mimeType == "image/gif" ||
		mimeType == "image/bmp" ||
		mimeType == "image/x-icon" ||
		mimeType == "image/webp"
}

func isAttachmentMimeType(mimeType string) bool {
	return mimeType == "application/x-gzip" ||
		mimeType == "application/zip" ||
		mimeType == "application/x-rar-compressed" ||
		mimeType == "application/pdf" ||
		mimeType == "audio/basic" ||
		mimeType == "audio/aiff" ||
		mimeType == "audio/mpeg" ||
		mimeType == "application/ogg" ||
		mimeType == "audio/midi" ||
		mimeType == "video/avi" ||
		mimeType == "audio/wave" ||
		mimeType == "video/webm" ||
		mimeType == "font/ttf" ||
		mimeType == "font/otf" ||
		mimeType == "font/collection" ||
		mimeType == "font/woff" ||
		mimeType == "font/woff2" ||
		mimeType == "application/wasm" ||
		mimeType == "application/postscript" ||
		mimeType == "application/vnd.ms-fontobject" ||
		mimeType == "application/octet-stream"
}

func UploadsDownloadHandler(c echo.Context) error {
	authUser := c.Get("authUser").(*database.User)
	db := c.Get("database").(*database.DkfDB)
	filename := c.Param("filename")
	file, err := db.GetUploadByFileName(filename)
	if err != nil {
		return c.Render(http.StatusOK, "standalone.upload404", nil)
	}
	if !file.Exists() {
		logrus.Error(filename + " does not exists")
		return c.Render(http.StatusOK, "standalone.upload404", nil)
	}

	if file.FileSize < config.MaxFileSizeBeforeDownload {
		fi, decFileBytes, err := file.GetContent()
		if err != nil || fi.IsDir() {
			return c.Render(http.StatusOK, "standalone.upload404", nil)
		}
		buf := bytes.NewReader(decFileBytes)

		// Validate image type and determine extension
		mimeType, err := getFileContentType(buf)
		_, err = buf.Seek(0, io.SeekStart)
		if err != nil {
			return c.Render(http.StatusOK, "standalone.upload404", nil)
		}

		// Serve images
		if isImageMimeType(mimeType) {
			http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
			return nil
		}

		if mimeType == "application/octet-stream" && utf8.Valid(decFileBytes) {
			mimeType = "text/plain; charset=utf-8"
		}

		// MimeType that always trigger a file "download"
		if isAttachmentMimeType(mimeType) {
			// Keep track of user downloads
			if _, err := db.CreateDownload(authUser.ID, filename); err != nil {
				logrus.Error(err)
			}
			c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", file.OrigFileName))
			http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
			return nil
		}

		// Serve any other file as text/plain
		c.Response().Header().Set(echo.HeaderContentType, "text/plain")
		c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("filename=%q", file.OrigFileName))
		http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
		return nil
	}

	userNbDownloaded := db.UserNbDownloaded(authUser.ID, filename)

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
	if _, err := db.CreateDownload(authUser.ID, filename); err != nil {
		logrus.Error(err)
	}

	fi, decFileBytes, err := file.GetContent()
	if err != nil {
		return c.Render(http.StatusOK, "standalone.upload404", nil)
	}
	buf := bytes.NewReader(decFileBytes)

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("%s; filename=%q", "attachment", file.OrigFileName))
	http.ServeContent(c.Response(), c.Request(), fi.Name(), fi.ModTime(), buf)
	return nil
}

func getFileContentType(out io.ReadSeeker) (string, error) {

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	_, err := out.Read(buffer)
	if err != nil {
		return "", err
	}

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	return contentType, nil
}
