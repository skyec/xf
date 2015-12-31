package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

const DEFAULT_TCP_SERVER_IP = "127.0.0.1:9090"
const DEFAULT_DATA_DIR = "/tmp/xfex1/tcpclient"
const DATA_FILE_NAME = "datafile"
const META_FILE_NAME = "meta.json"
const CHUNK_DIR = "chunks"
const DEFAULT_CONCURRENT = 5

var TCPServerIP string
var dataDir string
var chunkDir string
var concurrentChunks int

func main() {

	flag.StringVar(&TCPServerIP, "tcp-server", DEFAULT_TCP_SERVER_IP, "TCP Server address and port to connect to")
	flag.StringVar(&dataDir, "data-dir", DEFAULT_DATA_DIR, "Directory where the client will store its data")
	flag.IntVar(&concurrentChunks, "cc", DEFAULT_CONCURRENT, "Concurrent chunk downloads")
	flag.Parse()

	chunkDir = fmt.Sprintf("%s/%s", dataDir, CHUNK_DIR)
	err := os.MkdirAll(chunkDir, 0755)
	if err != nil {
		log.Fatalf("error making the data dir %s: %s", chunkDir, err)
	}

	client := &http.Client{}

	mf := newMetaFile(fmt.Sprintf("%s/%s", dataDir, META_FILE_NAME))
	err = mf.FetchMetafile(client, TCPServerIP)
	if err != nil {
		log.Fatal("error fetching the meta file: %s", err)
	}

	err = mf.Save()
	if err != nil {
		log.Fatal("error saving the metafile %s: %s", mf.fileName, err)
	}

	log.Println("SHA1:", mf.Sha1)
	log.Println("Chunks:", mf.Chunks)

	chChunks := make(chan int, mf.Chunks)
	for n := 0; n < mf.Chunks; n++ {
		chChunks <- n + 1
	}
	close(chChunks)

	wgChunks := &sync.WaitGroup{}
	wgChunks.Add(concurrentChunks)
	for n := 0; n < concurrentChunks; n++ {
		c := n
		go func() {
			log.Println("Starting chunk fetcher:", n)
			for chunkN := range chChunks {
				log.Println("Chunk:", chunkN)
				err = getChunk(client, TCPServerIP, chunkN)
				if err != nil {
					log.Fatalf("Failed to fetch chunk %d: %s", chunkN, err)
				}

			}

			wgChunks.Done()
			log.Println("Exiting chunk fetcher:", c)
		}()

	}
	log.Println("Fetching chunks ....")
	wgChunks.Wait()
	log.Println("Done")
}

func getChunk(client *http.Client, serverIP string, chunkN int) error {

	chunkFileName := fmt.Sprintf("%s/%d", chunkDir, chunkN)
	fd, err := os.Create(chunkFileName)
	if err != nil {
		return err
	}

	chunkURL := fmt.Sprintf("http://%s/chunk/%d", serverIP, chunkN)
	resp, err := client.Get(chunkURL)
	if err != nil {
		return fmt.Errorf("error fetching chunk %s: %s", chunkURL, err)
	}
	_, err = io.Copy(fd, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing response body to chunk file %s: %s", chunkFileName, err)
	}
	return fd.Close()
}

type metaFile struct {
	Sha1     string `json:"sha1"`
	Chunks   int    `json:"chunks"`
	fileName string
}

func newMetaFile(fileName string) *metaFile {
	return &metaFile{
		fileName: fileName,
	}
}

func (mf *metaFile) Load() error {
	b, err := ioutil.ReadFile(mf.fileName)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, mf)
}

func (mf *metaFile) Save() error {
	b, err := json.Marshal(mf)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(mf.fileName, b, 0644)
}

func (mf *metaFile) FetchMetafile(client *http.Client, address string) error {
	reqString := fmt.Sprintf("http://%s/file", address)

	resp, err := client.Get(reqString)
	if err != nil {
		return fmt.Errorf("Error fetching metadata file: %s", err)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response body: %s", err)
	}

	return json.Unmarshal(b, mf)
}
