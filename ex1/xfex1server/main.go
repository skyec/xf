package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
)

const BASE_DIR = "/tmp/xfex1"
const DATA_FILE = "datafile"
const CHUNK_DIR = "chunks"
const CHUNK_SIZE = 1500

var DEBUG bool

var serverAddress string

func init() {
	if len(os.Getenv("DEBUG")) > 0 {
		DEBUG = true
	}
}

func main() {

	flag.StringVar(&serverAddress, "server", ":9090", "server address")
	flag.Parse()

	http.HandleFunc("/newtest", newTest)
	http.HandleFunc("/file", getMeta)
	http.HandleFunc("/file/", getFile)
	http.HandleFunc("/chunk/", getChunk)
	log.Printf("Listening on port %s ...", serverAddress)
	http.ListenAndServe(serverAddress, nil)

}

func getChunk(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s", r.URL.Path)
	chunkN, err := strconv.Atoi(path.Base(r.URL.Path))
	if err != nil {
		log.Printf("error parsing chunk number from path %s: %s", r.URL.Path, err)
		http.Error(w, "error parsing chunk number from path", http.StatusBadRequest)
		return
	}

	chunkFileName := fmt.Sprintf("%s/%s/%d", BASE_DIR, CHUNK_DIR, chunkN)

	fd, err := os.Open(chunkFileName)
	if err != nil && os.IsNotExist(err) {
		log.Println("Not found")
		http.Error(w, "Chunk not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("error opening chunk file %s: %s", chunkFileName, err)
		http.Error(w, "error opening chunk file", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/octet-stream")
	_, err = io.Copy(w, fd)
	if err != nil {
		log.Printf("error writing chunk to client: %s", err)
	}
}

func getFile(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s", r.URL.Path)
	fd, err := os.Open(fmt.Sprintf("%s/%s", BASE_DIR, DATA_FILE))
	if err != nil {
		log.Printf("error opening datafile: %s", err)
		http.Error(w, "error opening datafile", http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/octet-stream")
	_, err = io.Copy(w, fd)
	if err != nil {
		log.Printf("Error writing data file to client: %s", err)
	}
}

func getMeta(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s", r.URL.Path)
	mf := newMetaFile(BASE_DIR)
	err := mf.Load()
	if err != nil {
		log.Printf("error loading the metafile: %s", err)
		http.Error(w, "error loading the metafil", http.StatusInternalServerError)
	}
	w.Header().Add("Content-Type", "application/json")
	_, err = w.Write(mf.Bytes())
	if err != nil {
		log.Printf("error writing meta file json to client: %s", err)
	}
}

type chunkWriter struct {
	dir        string
	dirInit    bool
	chunkCount int64
	byteCount  int64
	chunk      struct {
		fd      *os.File
		written int64
	}
}

func newChunkWriter(dir string) *chunkWriter {
	return &chunkWriter{
		dir: dir,
	}
}

func (cw *chunkWriter) initChunkFile() error {
	chunkN := cw.chunkCount + 1
	fd, err := os.Create(fmt.Sprintf("%s/%d", cw.dir, chunkN))
	if err != nil {
		return fmt.Errorf("error creating chunk file %d: %s", chunkN, err)
	}
	cw.chunkCount = chunkN
	cw.chunk.fd = fd
	cw.chunk.written = 0
	return nil
}

func (cw *chunkWriter) Write(b []byte) (int, error) {
	if DEBUG {
		log.Printf("Chunk write (%d)\n", len(b))
	}

	if cw.dirInit == false {
		log.Println("Init chunk dir:", cw.dir)
		err := os.MkdirAll(cw.dir, 0755)
		if err != nil {
			return 0, fmt.Errorf("error creating chunk dir: %s", err)
		}
		cw.dirInit = true
	}

	// initialize the chunk fd
	if cw.chunk.fd == nil {
		log.Println("Init first chunk file")
		err := cw.initChunkFile()
		if err != nil {
			return 0, err
		}
	}

	written := 0
	sz := CHUNK_SIZE - cw.chunk.written
	var chunk []byte
	if int64(len(b)) <= sz {
		chunk = b
	} else {
		chunk = b[:sz]
	}

	for written < len(b) {
		if DEBUG {
			log.Println("Write chunk:", len(chunk))
		}
		out := bytes.NewBuffer(chunk)
		n, err := io.Copy(cw.chunk.fd, out)
		if err != nil {
			return written, err
		}
		cw.chunk.written += n
		written += int(n)

		// DRY
		if cw.chunk.written == CHUNK_SIZE {

			err := cw.chunk.fd.Close()
			if err != nil {
				return written, fmt.Errorf("error closing the chunk file: %s", err)
			}

			err = cw.initChunkFile()
			if err != nil {
				return written, err
			}
			sz = CHUNK_SIZE

			if DEBUG {
				log.Println("Init the next chunk file:", cw.chunkCount)
			}
		}

		if len(b)-written < int(sz) {
			chunk = b[written:]
		} else {
			chunk = b[written : written+int(sz)]
		}
	}
	cw.byteCount += int64(written)
	return written, nil
}

type sha1Reader struct {
	in   io.Reader
	sha1 hash.Hash
}

func (sr *sha1Reader) Read(b []byte) (int, error) {
	n, err := sr.in.Read(b)
	if err != nil {
		return n, err
	}
	sr.sha1.Write(b)

	return n, nil
}

func (sr *sha1Reader) String() string {
	return fmt.Sprintf("%x", sr.sha1.Sum(nil))
}

func newSha1Reader(r io.Reader) *sha1Reader {
	return &sha1Reader{
		in:   r,
		sha1: sha1.New(),
	}
}

func newTest(w http.ResponseWriter, r *http.Request) {
	log.Printf("GET %s", r.URL.Path)

	in, err := os.Open("/dev/random")
	if err != nil {
		log.Println("Failed to open /dev/random: ", err)
		http.Error(w, "Failed to open /dev/random", http.StatusInternalServerError)
		return
	}
	inSha1 := newSha1Reader(in)

	err = os.MkdirAll(BASE_DIR, 0755)
	if err != nil {
		log.Println("Error creating base dir:", err)
		http.Error(w, "Error creating base dir", http.StatusInternalServerError)
		return
	}

	log.Println("Generating a new data file ...")
	data_file := fmt.Sprintf("%s/%s", BASE_DIR, DATA_FILE)

	dataFileFd, err := os.Create(data_file)
	if err != nil {
		log.Println("Failed to create the data file: %s: %s", data_file, err)
		http.Error(w, "Failed to create data file", http.StatusInternalServerError)
		return
	}

	chunks := newChunkWriter(fmt.Sprintf("%s/%s", BASE_DIR, CHUNK_DIR))
	out := io.MultiWriter(dataFileFd, chunks)

	_, err = io.CopyN(out, inSha1, 1024*1024*10)
	if err != nil {
		log.Println("Error copying data:", err)
		http.Error(w, "Error copying data", http.StatusInternalServerError)
		return
	}
	log.Println("File generated:", data_file)
	log.Println("SHA1:", inSha1)
	log.Println("Chunks:", chunks.dir)
	log.Println("NChunks:", chunks.chunkCount)

	mf := newMetaFile(BASE_DIR)
	mf.Sha1 = inSha1.String()
	mf.Chunks = chunks.chunkCount
	mf.Size = chunks.byteCount

	err = mf.Save()
	if err != nil {
		log.Printf("Error saving meta file: %s", err)
		http.Error(w, "Error saving meta file", http.StatusInternalServerError)
	}
	w.Write(mf.Bytes())
}

const META_FILE_NAME = "meta.json"

type metaFile struct {
	Sha1   string `json:"sha1"`
	Chunks int64  `json:"chunks"`
	Size   int64  `json:"size"`
	dir    string
}

func newMetaFile(dir string) *metaFile {
	return &metaFile{
		dir: dir,
	}
}

func (mf *metaFile) Bytes() []byte {
	b, err := json.Marshal(mf)
	if err != nil {
		log.Printf("error marshalling meta file data!!")
		panic("Outa here!")
	}
	return b
}

func (mf *metaFile) String() string {
	return string(mf.Bytes())
}

func (mf *metaFile) Save() error {
	return ioutil.WriteFile(fmt.Sprintf("%s/%s", mf.dir, META_FILE_NAME), mf.Bytes(), 0644)
}

func (mf *metaFile) Load() error {
	b, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", mf.dir, META_FILE_NAME))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, mf)
}
