package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
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
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; rv:102.0) Gecko/20100101 Firefox/102.0"
	dkfBaseURL    = "http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion"
	localhostAddr = "http://127.0.0.1:8080"
	torProxyAddr  = "127.0.0.1:9050"
)

var chunksCompleted int64 // atomic

func main() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	var nbThreads int
	var filedropUUID string
	var fileName string
	var dry bool
	var isLocal bool
	var maxChunkSize int64
	var httpTimeout time.Duration
	filedropUUIDUsage := "dkf filedrop uuid"
	fileNameUsage := "file to upload"
	nbThreadsUsage := "nb threads"
	nbThreadsDefaultValue := 20
	chunkSizeUsage := "chunk size"
	chunkSizeDefaultValue := int64(2 << 20) // 2MB
	flag.DurationVar(&httpTimeout, "http-timeout", 2*time.Minute, "http timeout")
	flag.StringVar(&filedropUUID, "uuid", "", filedropUUIDUsage)
	flag.StringVar(&filedropUUID, "u", "", filedropUUIDUsage)
	flag.StringVar(&fileName, "file", "", fileNameUsage)
	flag.StringVar(&fileName, "f", "", fileNameUsage)
	flag.IntVar(&nbThreads, "threads", nbThreadsDefaultValue, nbThreadsUsage)
	flag.IntVar(&nbThreads, "t", nbThreadsDefaultValue, nbThreadsUsage)
	flag.Int64Var(&maxChunkSize, "chunk-size", chunkSizeDefaultValue, chunkSizeUsage)
	flag.Int64Var(&maxChunkSize, "c", chunkSizeDefaultValue, chunkSizeUsage)
	flag.BoolVar(&dry, "dry", false, "dry run")
	flag.BoolVar(&isLocal, "local", false, "localhost development")
	flag.Parse()

	baseUrl := Ternary(isLocal, localhostAddr, dkfBaseURL)

	f, err := os.Open(fileName)
	if err != nil {
		logrus.Fatal(err.Error())
	}
	fs, err := f.Stat()
	if err != nil {
		logrus.Fatal(err.Error())
	}

	// Calculate sha256 of file
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		logrus.Fatalln(err)
	}
	fileSha256 := hex.EncodeToString(h.Sum(nil))

	fileSize := fs.Size()
	nbChunks := int64(math.Ceil(float64(fileSize) / float64(maxChunkSize)))

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
		logrus.Debug("sending metadata")
		client := doGetClient(isLocal, httpTimeout)
		body := url.Values{}
		body.Set("init", "1")
		body.Set("fileName", fs.Name())
		body.Set("fileSize", strconv.FormatInt(fileSize, 10))
		body.Set("fileSha256", fileSha256)
		body.Set("chunkSize", strconv.FormatInt(maxChunkSize, 10))
		body.Set("nbChunks", strconv.FormatInt(nbChunks, 10))
		req, _ := http.NewRequest(http.MethodPost, baseUrl+"/file-drop/"+filedropUUID+"/dkfupload", strings.NewReader(body.Encode()))
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			logrus.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			logrus.Fatal(fmt.Errorf("invalid status code %s", resp.Status))
		}
		logrus.Debug("done sending metadata")
	}

	chunksCh := make(chan int64)
	wg := &sync.WaitGroup{}

	// Provide worker threads with tasks to do
	go func() {
		for chunkNum := int64(0); chunkNum < nbChunks; chunkNum++ {
			chunksCh <- chunkNum
		}
		// closing the channel will ensure all workers exit gracefully
		close(chunksCh)
	}()

	// Start worker threads
	wg.Add(nbThreads)
	for i := 0; i < nbThreads; i++ {
		go work(i, wg, chunksCh, isLocal, dry, maxChunkSize, nbChunks, f, baseUrl, filedropUUID, httpTimeout)
		time.Sleep(25 * time.Millisecond)
	}

	// Wait for all workers to have completed
	wg.Wait()

	if !dry {
		client := doGetClient(isLocal, httpTimeout)
		body := url.Values{}
		body.Set("completed", "1")
		req, _ := http.NewRequest(http.MethodPost, baseUrl+"/file-drop/"+filedropUUID+"/dkfupload", strings.NewReader(body.Encode()))
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			logrus.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			logrus.Fatal(fmt.Errorf("invalid status code %s", resp.Status))
		}
	}

	logrus.Infof("All done in %s\n", ShortDur(time.Since(start)))
}

func work(i int, wg *sync.WaitGroup, chunksCh chan int64, isLocal, dry bool, maxChunkSize, nbChunks int64, f *os.File, baseUrl, filedropUUID string, httpTimeout time.Duration) {
	client := doGetClient(isLocal, httpTimeout)

	buf := make([]byte, maxChunkSize)
	for chunkNum := range chunksCh {
		start := time.Now()
		offset := chunkNum * maxChunkSize
		n, _ := f.ReadAt(buf, offset)
		logrus.Infof("Thread #%03d | chunk #%03d | read %d | from %d to %d\n", i, chunkNum, n, offset, offset+int64(n))
		if !dry {
			hasToSucceed(func() error {
				partFileName := fmt.Sprintf("part_%d", chunkNum)

				// Ask server if he already has the chunk
				{
					body := url.Values{}
					body.Set("chunkFileName", partFileName)
					req, _ := http.NewRequest(http.MethodPost, baseUrl+"/file-drop/"+filedropUUID+"/dkfupload", strings.NewReader(body.Encode()))
					req.Header.Set("User-Agent", userAgent)
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					resp, err := client.Do(req)
					if err != nil {
						if os.IsTimeout(err) {
							logrus.Infof("Thread #%03d gets a new client\n", i)
							client = doGetClient(isLocal, httpTimeout)
						}
						return err
					}
					defer resp.Body.Close()
					// We use teapot status (because why not) to express that we already have the chunk
					if resp.StatusCode == http.StatusTeapot {
						logrus.Infof("Thread #%03d | server already has chunk #%03d; skip", i, chunkNum)
						return nil
					}
				}

				start = time.Now()
				body := new(bytes.Buffer)
				w := multipart.NewWriter(body)
				part, err := w.CreateFormFile("file", partFileName)
				if err != nil {
					return err
				}
				if _, err := part.Write(buf[:n]); err != nil {
					return err
				}
				if err := w.Close(); err != nil {
					return err
				}

				req, _ := http.NewRequest(http.MethodPost, baseUrl+"/file-drop/"+filedropUUID+"/dkfupload", body)
				req.Header.Set("User-Agent", userAgent)
				req.Header.Set("Content-Type", w.FormDataContentType())
				resp, err := client.Do(req)
				if err != nil {
					if os.IsTimeout(err) {
						logrus.Infof("Thread #%03d gets a new client\n", i)
						client = doGetClient(isLocal, httpTimeout)
					}
					return err
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("invalid status code %s", resp.Status)
				}
				return nil
			})
		}
		newChunksCompleted := atomic.AddInt64(&chunksCompleted, 1)
		logrus.Infof("Thread #%03d | chunk #%03d | completed in %s (%d/%d)\n", i, chunkNum, ShortDur(time.Since(start)), newChunksCompleted, nbChunks)
	}
	wg.Done()
}

func doGetClient(isLocal bool, httpTimeout time.Duration) (client *http.Client) {
	hasToSucceed(func() (err error) {
		if isLocal {
			client = http.DefaultClient
		} else {
			token := GenerateTokenN(8)
			if client, err = GetHttpClient(&proxy.Auth{User: token, Password: token}); err != nil {
				return err
			}
		}
		return
	})
	client.Timeout = httpTimeout
	return
}

// Will keep retrying a callback until no error is returned
func hasToSucceed(clb func() error) {
	waitTime := 5
	for {
		err := clb()
		if err == nil {
			break
		}
		logrus.Errorf("wait %d seconds before retry; %v\n", waitTime, err)
		time.Sleep(time.Duration(waitTime) * time.Second)
	}
}

// GetHttpClient http client that uses tor proxy
func GetHttpClient(auth *proxy.Auth) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", torProxyAddr, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tor proxy : %w", err)
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar : %w", err)
	}
	return &http.Client{Transport: transport, Jar: jar}, nil
}

// GenerateTokenN generates a random printable string from N bytes
func GenerateTokenN(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func Ternary[T any](predicate bool, a, b T) T {
	if predicate {
		return a
	}
	return b
}

func ShortDur(d time.Duration) string {
	if d < time.Minute {
		d = d.Round(time.Millisecond)
	} else {
		d = d.Round(time.Second)
	}
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
