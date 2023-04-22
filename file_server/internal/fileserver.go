package internal

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// You may NOT change anything in this file (or any of the go files)
const (
	maxConnections        = 15
	baseLatencyPerRequest = 333 // # of ms added for all requests
	port                  = 1234
)

func NewFileServer() *FileServer {
	return &FileServer{connections: 0,
		knownFiles: map[string]bool{},
		inProcess:  make(map[string]bool),
	}
}

type FileServer struct {
	connections   int
	knownFiles    map[string]bool
	inProcess     FileSet
	fileLock      sync.RWMutex
	inProcessLock sync.RWMutex
	connLock      sync.RWMutex
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
		fs.WriteResponseBody(response, "Too many requests. Slow down.")
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
		fs.WriteResponseBody(response, "File name is empty.")
		return
	}

	// Mark file in process so other FS ops for this file wait behind it
	fs.waitForOpenInProcess(fileName)
	defer fs.removeInProcessLock(fileName)

	fs.fileLock.RLock()
	_, hasFile := fs.knownFiles[fileName]
	if !hasFile {
		// If file not found in known file cache, check fs directly in case file was written by different process.
		_, err := os.Stat(filePath)
		if err != nil {
			log.Errorf("File not found err: %+v", err)
			response.WriteHeader(http.StatusNotFound)
			fs.WriteResponseBody(response, "File not found.")
			fs.fileLock.RUnlock()
			return
		}

		// No err, file must exist, add it to known files
		fs.fileLock.RUnlock()
		fs.fileLock.Lock()
		fs.knownFiles[fileName] = true
		fs.fileLock.Unlock()
	} else {
		defer fs.fileLock.RUnlock()
	}

	fs.inProcess.Add(filePath)
	// Read file from FS
	file, err := os.Open(filePath)
	defer file.Close()
	if err != nil {
		log.Errorf("Failed to read file: %s. Error: %+v", filePath, err)
		fs.WriteResponseBody(response, err.Error())
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		log.Errorf("Failed to read file stat for file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, err.Error())
		return
	}
	numBytes := stat.Size()

	// Set header type
	response.Header().Set("Content-Type", "application/octet-stream")

	// Copy data
	written, err := io.Copy(response, file)
	if err != nil {
		log.Errorf("Get failed to read file bytes for file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, err.Error())
		return
	}

	// Verify correct amount of data written.
	if numBytes != written {
		log.Errorf("Invalid number of bytes written to response. Expected %d, got %d", numBytes, written)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, "Data write corruption. Please retry")
		return
	}

	return
}

func (fs *FileServer) HandlePut(response http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// Throttle if > maxConnections
	if !fs.CanTakeConnection() {
		response.WriteHeader(http.StatusTooManyRequests)
		fs.WriteResponseBody(response, "Too many requests. Slow down.")
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
		fs.WriteResponseBody(response, "No file name provided")
		return
	}

	// Mark file in process so other FS ops for this file wait behind it
	fs.waitForOpenInProcess(fileName)
	defer fs.removeInProcessLock(fileName)

	// Open file for writing
	file, err := os.Create(filePath)
	defer file.Close()
	if err != nil {
		log.Errorf("Failed to create file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, err.Error())
		return
	}

	// Copy data
	written, err := io.Copy(file, request.Body)
	if err != nil {
		log.Errorf("Failed to read file bytes for file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, err.Error())
		_ = os.Remove(filePath)
		return
	}

	// Verify correct amount of data written.
	if request.ContentLength != written {
		log.Errorf("Invalid number of bytes written to response. Expected %d, got %d", request.ContentLength, written)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, "Write corruption, please retry.")
		_ = os.Remove(filePath)
		return
	}

	fs.fileLock.Lock()
	defer fs.fileLock.Unlock()

	// Write successful response
	fs.knownFiles[fileName] = true
	response.WriteHeader(http.StatusCreated)
	return
}

func (fs *FileServer) HandleDelete(response http.ResponseWriter, request *http.Request, params httprouter.Params) {
	// Throttle if > maxConnections
	if !fs.CanTakeConnection() {
		response.WriteHeader(http.StatusTooManyRequests)
		fs.WriteResponseBody(response, "Too many requests. Slow down.")
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
		fs.WriteResponseBody(response, "No file name specified.")
		return
	}

	fs.fileLock.Lock()
	defer fs.fileLock.Unlock()

	// Mark file in process so other FS ops for this file wait behind it
	fs.waitForOpenInProcess(fileName)
	defer fs.removeInProcessLock(fileName)

	_, err := os.Stat(filePath)
	if err != nil {
		delete(fs.knownFiles, fileName)
		response.WriteHeader(http.StatusOK)
		fs.WriteResponseBody(response, "File not found. Already deleted.")
		return
	}

	// Open file for writing
	err = os.Remove(filePath)
	if err != nil {
		log.Errorf("Failed to delete file: %s. Error: %+v", filePath, err)
		response.WriteHeader(http.StatusInternalServerError)
		fs.WriteResponseBody(response, err.Error())
		return
	}

	// Write successful response
	delete(fs.knownFiles, fileName)
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

func (fs *FileServer) WriteResponseBody(response http.ResponseWriter, message string) {
	_, err := response.Write([]byte(message))
	if err != nil {
		log.Errorf("Failed to write response body: %+v", err)
	}
}

func (fs *FileServer) waitForOpenInProcess(fileName string) {
	jitter := rand.Intn(25)

	fs.inProcessLock.RLock()
	fileInProcess := fs.inProcess.Has(fileName)
	fs.inProcessLock.RUnlock()
	for fileInProcess {
		time.Sleep(time.Millisecond * time.Duration(jitter))
		fs.inProcessLock.RLock()
		fileInProcess = fs.inProcess.Has(fileName)
		fs.inProcessLock.RUnlock()
	}

	fs.inProcessLock.Lock()
	defer fs.inProcessLock.Unlock()
	fs.inProcess.Add(fileName)
}

func (fs *FileServer) removeInProcessLock(fileName string) {
	fs.inProcessLock.Lock()
	fs.inProcess.Delete(fileName)
	fs.inProcessLock.Unlock()
}
