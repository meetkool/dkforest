package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"golang.org/x/net/proxy"
)

const (
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; rv:102.0) Gecko/20100101 Firefox/102.0"
	dkfBaseURL    = "http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion"
	localhostAddr = "http://127.0.0.1:8080"
	torProxyAddr  = "127.0.0.1:9050"
)

func main() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	var nbThreads int
	var filedropUUID, fileName string
	var dry, isLocal bool
	var maxChunkSize int64
	var httpTimeout time.Duration

	flag.DurationVar(&httpTimeout, "http-timeout", 2*time.Minute, "http timeout")
	flag.StringVar(&filedropUUID, "uuid", "", "dkf filedrop uuid")
	flag.StringVar(&filedropUUID, "u", "", "dkf filedrop uuid")
	flag.StringVar(&fileName, "file", "", "file to upload")
	flag.StringVar(&fileName, "f", "", "file to upload")
	flag.IntVar(&nbThreads, "threads", 20, "nb threads")
	flag.IntVar(&nbThreads, "t", 20, "nb threads")
	flag.Int64Var(&maxChunkSize, "chunk-size", 2<<20, "chunk size")
	flag.Int64Var(&maxChunkSize, "c", 2<<20, "chunk size")
	flag.BoolVar(&dry, "dry", false, "dry run")
	flag.BoolVar(&isLocal, "local", false, "localhost development")

	err := flag.Parse()
	if err != nil {
		logrus.Fatalf("Error parsing flags: %v", err)
	}

	baseUrl := Ternary(isLocal, localhostAddr, dkfBaseURL)

	f, err := os.Open(fileName)
	if err != nil {
		logrus.Fatalf("Error opening file: %v", err)
	}
	defer f.Close()

	fs, err := f.Stat()
	if err != nil {
		logrus.Fatalf("Error getting file stats: %v", err)
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logrus.Fatalf("Error calculating sha256: %v", err)
	}
	fileSha256 := hex.EncodeToString(h.Sum(nil))

	fileSize := fs.Size()
	nbChunks := int64(math.Ceiling(float64(fileSize) / float64(maxChunkSize)))

	// Print out information about the file
	{
		logrus.Infof("filedrop UUID: %s\n", filedropUUID)
		logrus.Infof("         file: %s\n", fs.Name())
		logrus.Infof("       sha256: %s\n", fileSha256)
		logrus.Infof("    file size: %s (%s)\n", humanize.Bytes(uint64(fileSize)), humanize.Comma(fileSize))
		logrus.Infof("  chunks size: %s (%s)\n", humanize.Bytes(uint64(maxChunkSize)), humanize.Comma(maxChunkSize))
		logrus.Infof("    nb chunks: %d\n", nbChunks)
		logrus.Infof("   nb threads: %d\n", nbThreads)
		logrus.Infof(" http timeout: %s\n", ShortDur(httpTimeout))
		if dry {
			logrus.Infof("      dry run: %t\n", dry)
		}
		logrus.Infof(strings.Repeat("-", 80))
	}

	start := time.Now()

	// Init the filedrop and send metadata about the file
	if !dry {
		client := doGetClient(isLocal, httpTimeout)
		body := url.Values{}
		body.Set("init", "1")
		body.Set("fileName", fs.Name())
		body.Set("fileSize", strconv.FormatInt(fileSize, 10))
		body.Set("fileSha256", fileSha256)
		body.Set("chunkSize", strconv.FormatInt(maxChunkSize, 10))
		body.Set("nbChunks", strconv.Format
