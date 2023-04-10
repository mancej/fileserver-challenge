package internal

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	maxConnections        = 25
	baseLatencyPerRequest = 750 // # ms added for all requests
	port                  = 1234
)

func NewFileServer() *FileServer {
	return &FileServer{connections: 0, knownFiles: map[string]bool{}}
}

type FileServer struct {
	connections int
	knownFiles  map[string]bool
	fileLock    sync.RWMutex
	connLock    sync.RWMutex
}

func (fs *FileServer) Run() error {
	router := httprouter.New()
	router.GET("/api/fileserver/:filename", fs.HandleGet)
	router.PUT("/api/fileserver/:filename", fs.HandlePut)
	router.DELETE("/api/fileserver/:filename", fs.HandleDelete)

	return http.ListenAndServe(fmt.Sprintf(":%d", port), router)
}

func (fs *FileServer) SimulateLatency() {
	time.Sleep(baseLatencyPerRequest * time.Millisecond)
}

func (fs *FileServer) HandleGet(response http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// Throttle if > maxConnections
	if !fs.CanTakeConnection() {
		response.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// Consume connection
	fs.IncrementConnection()
	defer fs.DecrementConnection()
	fs.SimulateLatency()

	fileName := params.ByName("filename")
	filePath := fmt.Sprintf("/tmp/%s", fileName)
	defer request.Body.Close()

	if fileName == "" {
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	fs.fileLock.RLock()
	_, hasFile := fs.knownFiles[fileName]
	if !hasFile {
		response.WriteHeader(http.StatusNotFound)
		fs.fileLock.RUnlock()
		return
	}
	fs.fileLock.RUnlock()

	// Read file from FS
	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("Failed to read file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		log.Errorf("Failed to read file stat for file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	numBytes := stat.Size()

	// Set header type
	response.Header().Set("Content-Type", "application/octet-stream")

	// Copy data
	written, err := io.Copy(response, file)
	if err != nil {
		log.Errorf("Failed to read file bytes for file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Verify correct amount of data written.
	if numBytes != written {
		log.Errorf("Invalid number of bytes written to response. Expected %d, got %d", numBytes, written)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write successful response
	response.WriteHeader(http.StatusOK)
	return
}

func (fs *FileServer) HandlePut(response http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// Throttle if > maxConnections
	if !fs.CanTakeConnection() {
		response.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// Consume connection
	fs.IncrementConnection()
	defer fs.DecrementConnection()
	fs.SimulateLatency()

	fileName := params.ByName("filename")
	filePath := fmt.Sprintf("/tmp/%s", fileName)
	defer request.Body.Close()

	log.Infof("Saving file: %s", filePath)

	if fileName == "" {
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	fs.fileLock.Lock()
	fs.knownFiles[fileName] = true
	fs.fileLock.Unlock()

	// Open file for writing
	file, err := os.Create(filePath)
	if err != nil {
		log.Errorf("Failed to create file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Copy data
	written, err := io.Copy(file, request.Body)
	if err != nil {
		log.Errorf("Failed to read file bytes for file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Verify correct amount of data written.
	if request.ContentLength != written {
		log.Errorf("Invalid number of bytes written to response. Expected %d, got %d", request.ContentLength, written)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write successful response
	response.WriteHeader(http.StatusCreated)
	return
}

func (fs *FileServer) HandleDelete(response http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// Throttle if > maxConnections
	if !fs.CanTakeConnection() {
		response.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// Consume connection
	fs.IncrementConnection()
	defer fs.DecrementConnection()
	fs.SimulateLatency()

	fileName := params.ByName("filename")
	filePath := fmt.Sprintf("/tmp/%s", fileName)
	defer request.Body.Close()

	if fileName == "" {
		response.WriteHeader(http.StatusBadRequest)
		return
	}

	fs.fileLock.Lock()
	delete(fs.knownFiles, fileName)
	fs.fileLock.Unlock()

	// Open file for writing
	err := os.Remove(filePath)
	if err != nil {
		log.Errorf("Failed to delete file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write successful response
	response.WriteHeader(http.StatusOK)
}

func (fs *FileServer) CanTakeConnection() bool {
	// Throttle if > maxConnections
	fs.connLock.RLock()
	defer fs.connLock.RUnlock()

	return fs.connections < maxConnections
}

func (fs *FileServer) IncrementConnection() {
	fs.connLock.Lock()
	fs.connections = fs.connections + 1
	fs.connLock.Unlock()
}
func (fs *FileServer) DecrementConnection() {
	fs.connLock.Lock()
	fs.connections = fs.connections - 1
	fs.connLock.Unlock()
}
