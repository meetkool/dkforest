package web

import (
	"context"
	"fmt"
	"github.com/labstack/echo"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"github.com/ulule/limiter"
	"github.com/ulule/limiter/drivers/store/memory"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v1"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func getMainServer(db *database.DkfDB, i18nBundle *i18n.Bundle, renderer *tmp.Templates, clientFE clientFrontends.ClientFrontend) echo.HandlerFunc {
	// ... (same as original code)
}

func getBaseServer(db *database.DkfDB, clientFE clientFrontends.ClientFrontend) *echo.Echo {
	// ... (same as original code)
}

func getSubdomainServer(db *database.DkfDB, clientFE clientFrontends.ClientFrontend) *echo.Echo {
	// ... (same as original code)
}

func getI2pServer(db *database.DkfDB) *echo.Echo {
	// ... (same as original code)
}

func getTorServer(db *database.DkfDB) *echo.Echo {
	// ... (same as original code)
}

func Start(db *database.DkfDB, host string, port int) {
	// ... (same as original code)
}

func extractGlobalCircuitIdentifier(m string) int64 {
	// ... (same as original code)
}

func getReverseProxy(u string) *httputil.ReverseProxy {
	// ... (same as original code)
}

func newEcho() *echo.Echo {
	// ... (same as original code)
}

func configTorProdServer(e *echo.Echo) {
	// ... (same as original code)
}

func getI18nBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	dir, err := config.LocalsFs.ReadDir(".")
	if err != nil {
		logrus.Fatalf("failed to read locals directory: %v", err)
	}

	fileNames := make([]string, 0)
	for _, d := range dir {
		fileNames = append(fileNames, d.Name())
	}

	for _, fileName := range fileNames {
		if strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, "sample.yaml") {
			file, err := config.LocalsFs.ReadFile(fileName)
			if err != nil {
				logrus.Fatalf("failed to read i18n file %s: %v", fileName, err)
			}
			_, err = bundle.ParseMessageFileBytes(file, fileName)
			if err != nil {
				logrus.Fatalf("failed to parse i18n file %s: %v", fileName, err)
			}
		}
	}

	if err := utils.LoadLocals(bundle); err != nil {
		logrus.Fatalf("failed to load locales: %v", err)
	}
	return bundle
}

