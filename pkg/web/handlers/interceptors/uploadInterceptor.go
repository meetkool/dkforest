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
		defer file.Close()
		if file == nil || handler == nil {
			return
	
