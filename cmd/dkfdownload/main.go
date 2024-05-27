package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
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

	var apiKey string
	var nbThreads int
	var filedropUUID string
	var isLocal bool
	var httpTimeout time.Duration
	apiKeyUsage := "api key"
	filedropUUIDUsage := "dkf filedrop uuid"
	nbThreadsUsage := "nb threads"
	nbThreadsDefaultValue := 20
	flag.StringVar(&apiKey, "api-key", "", apiKeyUsage)
	flag.StringVar(&apiKey, "a", "", apiKeyUsage)
	flag.DurationVar(&httpTimeout, "http-timeout", 2*time.Minute, "http timeout")
	flag.StringVar(&filedropUUID, "uuid", "", filedropUUIDUsage)
	flag.StringVar(&filedropUUID, "u", "", filedropUUIDUsage)
	flag.IntVar(&nbThreads, "threads", nbThreadsDefaultValue, nbThreadsUsage)
	flag.IntVar(&nbThreads, "t", nbThreadsDefaultValue, nbThreadsUsage)
	flag.BoolVar(&isLocal, "local", false, "localhost development")
	flag.Parse()

	baseUrl := Ternary(isLocal, localhostAddr, dkfBaseURL)
	endpoint := baseUrl + "/api/v1/file-drop/" + filedropUUID + "/dkfdownload"

	client := doGetClient(isLocal, httpTimeout)

	// Download metadata file
	by, err := os.ReadFile(filepath.Join(filedropUUID, "metadata"))
	if err != nil {
		body := url.Values{}
		body.Set("init", "1")
		req, _ := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body.Encode()))
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("DKF_API_KEY", apiKey)
		resp, err := client.Do(req)
		if err != nil {
			logrus.Fatalln(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			logrus.Fatalln(resp.Status)
		}
		by, _ = io.ReadAll(resp.Body)

		_ = os.Mkdir(filedropUUID, 0755)
		_ = os.WriteFile(filepath.Join(filedropUUID, "metadata"), by, 0644)
	}

	// Read metadata information
	lines := strings.Split(string(by), "\n")
	origFileName := lines[0]
	password, _ := base64.StdEncoding.DecodeString(lines[1])
	iv, _ := base64.StdEncoding.DecodeString(lines[2])
	fileSha256 := lines[3]
	fileSize, _ := strconv.ParseInt(lines[4], 10, 64)
	nbChunks, _ := strconv.ParseInt(lines[5], 10, 64)

	// Print out information about the file
	{
		logrus.Infof("filedrop UUID: %s\n", filedropUUID)
		logrus.Infof("         file: %s\n", origFileName)
		logrus.Infof("       sha256: %s\n", fileSha256)
		logrus.Infof("    file size: %s (%s)\n", humanize.Bytes(uint64(fileSize)), humanize.Comma(fileSize))
		logrus.Infof("    nb chunks: %d\n", nbChunks)
		logrus.Infof("   nb threads: %d\n", nbThreads)
		logrus.Infof(" http timeout: %s\n", ShortDur(httpTimeout))
		logrus.Infof(strings.Repeat("-", 80))
	}

	start := time.Now()

	chunksCh := make(chan int64)

	// Provide worker threads with tasks to do
	go func() {
		for chunkNum := int64(0); chunkNum < nbChunks; chunkNum++ {
			chunksCh <- chunkNum
		}
		// closing the channel will ensure all workers exit gracefully
		close(chunksCh)
	}()

	// Download every chunks
	wg := &sync.WaitGroup{}
	wg.Add(nbThreads)
	for i := 0; i < nbThreads; i++ {
		go work(i, wg, endpoint, filedropUUID, apiKey, chunksCh, isLocal, httpTimeout, nbChunks)
		time.Sleep(25 * time.Millisecond)
	}
	wg.Wait()
	logrus.Infof("all chunks downloaded in %s", ShortDur(time.Since(start)))

	// Get sorted chunks file names
	dirEntries, _ := os.ReadDir(filedropUUID)
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

	// Create final downloaded file
	outFile, err := os.OpenFile(filepath.Join(filedropUUID, origFileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		logrus.Fatalln(err)
	}
	defer outFile.Close()

	// Decryption stream
	block, err := aes.NewCipher(password)
	if err != nil {
		logrus.Fatalln(err)
	}
	stream := cipher.NewCTR(block, iv)

	h := sha256.New()

	logrus.Info("Decrypting final file & hash")
	for _, fileName := range fileNames {
		// Read chunk file
		by, err := os.ReadFile(filepath.Join(filedropUUID, fileName))
		if err != nil {
			logrus.Fatalln(err)
		}
		h.Write(by)
		// Decrypt chunk file
		dst := make([]byte, len(by))
		stream.XORKeyStream(dst, by)
		// Write to final file
		_, err = outFile.Write(dst)
	}

	// Ensure downloaded file sha256 is correct
	newFileSha256 := hex.EncodeToString(h.Sum(nil))
	if newFileSha256 == fileSha256 {
		logrus.Infof("downloaded sha256 is valid %s", newFileSha256)
	} else {
		logrus.Fatalf("downloaded sha256 doesn't match %s != %s", newFileSha256, fileSha256)
	}

	// Cleanup
	logrus.Info("cleanup chunks files")
	for _, chunkFileName := range fileNames {
		_ = os.Remove(filepath.Join(filedropUUID, chunkFileName))
	}
	_ = os.Remove(filepath.Join(filedropUUID, "metadata"))

	logrus.Infof("all done in %s", ShortDur(time.Since(start)))
}

func work(i int, wg *sync.WaitGroup, endpoint, filedropUUID, apiKey string, chunksCh chan int64, isLocal bool, httpTimeout time.Duration, nbChunks int64) {
	defer wg.Done()
	client := doGetClient(isLocal, httpTimeout)
	for chunkNum := range chunksCh {
		start := time.Now()
		logrus.Infof("thread #%03d | chunk #%03d", i, chunkNum)
		hasToSucceed(func() error {
			start = time.Now()
			body := url.Values{}
			body.Set("chunk", strconv.FormatInt(chunkNum, 10))
			req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body.Encode()))
			if err != nil {
				return err
			}
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("DKF_API_KEY", apiKey)
			resp, err := client.Do(req)
			if err != nil {
				if os.IsTimeout(err) {
					logrus.Infof("thread #%03d gets a new client\n", i)
					client = doGetClient(isLocal, httpTimeout)
				}
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				logrus.Fatalln(resp.Status)
			} else if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("thread #%03d | chunk #%03d | invalid status code %s", i, chunkNum, resp.Status)
			}
			f, err := os.OpenFile(filepath.Join(filedropUUID, "part_"+strconv.FormatInt(chunkNum, 10)), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err = io.Copy(f, resp.Body); err != nil {
				if os.IsTimeout(err) {
					logrus.Infof("thread #%03d gets a new client\n", i)
					client = doGetClient(isLocal, httpTimeout)
				}
				return err
			}
			return nil
		})
		newChunksCompleted := atomic.AddInt64(&chunksCompleted, 1)
		logrus.Infof("thread #%03d | chunk #%03d | completed in %s (%d/%d)\n", i, chunkNum, ShortDur(time.Since(start)), newChunksCompleted, nbChunks)
	}
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
