package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	userAgent     = "Mozilla/5.0 (Windows NT 10.0; rv:102.0) Gecko/20100101 Firefox/102.0"
	dkfBaseURL    = "http://dkforestseeaaq2dqz2uflmlsybvnq2irzn4ygyvu53oazyorednviid.onion"
	localhostAddr = "http://127.0.0.1:8080"
	torProxyAddr  = "127.0.0.1:9050"
)

func main() {
	var apiKey string
	var nbThreads int
	var filedropUUID string
	var isLocal bool
	var httpTimeout time.Duration

	flag.StringVar(&apiKey, "api-key", "", "api key")
	flag.StringVar(&apiKey, "a", "", "api key")
	flag.DurationVar(&httpTimeout, "http-timeout", 2*time.Minute, "http timeout")
	flag.StringVar(&filedropUUID, "uuid", "", "dkf filedrop uuid")
	flag.StringVar(&filedropUUID, "u", "", "dkf filedrop uuid")
	flag.IntVar(&nbThreads, "threads", 20, "nb threads")
	flag.IntVar(&nbThreads, "t", 20, "nb threads")
	flag.BoolVar(&isLocal, "local", false, "localhost development")
	flag.Parse()

	baseUrl := decideBaseURL(isLocal)
	endpoint := baseUrl + "/api/v1/file-drop/" + filedropUUID + "/dkfdownload"

	client := getClient(isLocal, httpTimeout)

	// Download metadata file
	metadata, err := downloadMetadata(client, endpoint, apiKey, filedropUUID)
	if err != nil {
		log.Fatalf("failed to download metadata: %v", err)
	}

	// Read metadata information
	origFileName, password, iv, fileSha256, fileSize, nbChunks, err := parseMetadata(metadata)
	if err != nil {
		log.Fatalf("failed to parse metadata: %v", err)
	}

	// Print out information about the file
	printInfo(origFileName, fileSha256, fileSize, nbChunks)

	start := time.Now()
	chunksCh := make(chan int64)
	var wg sync.WaitGroup

	// Download every chunks
	for i := 0; i < nbThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			downloadChunks(client, endpoint, apiKey, filedropUUID, chunksCh, isLocal, httpTimeout, nbChunks)
		}()
	}
	wg.Wait()
	log.Printf("all chunks downloaded in %s", shortDur(time.Since(start)))

	// Get sorted chunks file names
	fileNames := getSortedChunkFileNames(filedropUUID)

	// Create final downloaded file
	outFile, err := os.OpenFile(filepath.Join(filedropUUID, origFileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("failed to open output file: %v", err)
	}
	defer outFile.Close()

	// Decryption stream
	block, err := aes.NewCipher(password)
	if err != nil {
		log.Fatalf("failed to create cipher: %v", err)
	}
	stream := cipher.NewCTR(block, iv)

	h := sha256.New()

	log.Println("Decrypting final file & hash")
	for _, fileName := range fileNames {
		// Read chunk file
		chunkFile, err := os.ReadFile(filepath.Join(filedropUUID, fileName))
		if err != nil {
			log.Fatalf("failed to read chunk file: %v", err)
		}
		h.Write(chunkFile)
		// Decrypt chunk file
		dst := make([]byte, len(chunkFile))
		stream.XORKeyStream(dst, chunkFile)
		// Write to final file
		_, err = outFile.Write(dst)
		if err != nil {
			log.Fatalf("failed to write to output file: %v", err)
		}
	}

	// Ensure downloaded file sha256 is correct
	newFileSha256 := hex.EncodeToString(h.Sum(nil))
	if newFileSha256 == fileSha256 {
		log.Printf("downloaded sha256 is valid %s", newFileSha256)
	} else {
		log.Fatalf("downloaded sha256 doesn't match %s != %s", newFileSha256, fileSha256)
	}

	// Cleanup
	log.Println("cleanup chunks files")
	for _, chunkFileName := range fileNames {
		_ = os.Remove(filepath.Join(filedropUUID, chunkFileName))
	}
	_ = os.Remove(filepath.Join(filedropUUID, "metadata"))

	log.Printf("all
