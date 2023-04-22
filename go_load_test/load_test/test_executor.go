package load_test

import (
	"bytes"
	crand "crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type TestExecutor struct {
	client        *http.Client
	inProcess     FileSet
	maxFileSize   int64
	inProcessLock sync.RWMutex
	results       chan TestResult
	endpointCfg   TestEndpointConfig
	fileSizeLock  sync.RWMutex
}

func NewTestExecutor(client *http.Client, config TestEndpointConfig, testConfig TestConfig, resultsChan chan TestResult) *TestExecutor {
	return &TestExecutor{
		client:        client,
		endpointCfg:   config,
		inProcess:     make(map[string]bool),
		maxFileSize:   testConfig.MaxFileSize,
		inProcessLock: sync.RWMutex{},
		results:       resultsChan,
	}
}

func (tr *TestExecutor) waitForOpenInProcess(fileName string) {
	jitter := rand.Intn(100)

	tr.inProcessLock.RLock()
	fileInProcess := tr.inProcess.Has(fileName)
	tr.inProcessLock.RUnlock()
	for fileInProcess {
		time.Sleep(time.Millisecond * time.Duration(jitter))
		tr.inProcessLock.RLock()
		fileInProcess = tr.inProcess.Has(fileName)
		tr.inProcessLock.RUnlock()
	}

	tr.inProcessLock.Lock()
	defer tr.inProcessLock.Unlock()
	tr.inProcess.Add(fileName)
}

func (tr *TestExecutor) PutFile(fileName string) {
	tr.waitForOpenInProcess(fileName)
	defer func() {
		tr.inProcessLock.Lock()
		tr.inProcess.Delete(fileName)
		tr.inProcessLock.Unlock()
	}()
	fileSize := tr.randomFileSize()
	fileBytes := make([]byte, fileSize)
	_, err := crand.Read(fileBytes)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: PUT,
			response: nil,
			message:  "Failed to generate random file bytes",
			err:      err,
			failed:   true,
		}
		return
	}

	byteString := b64.StdEncoding.EncodeToString(fileBytes)
	req, err := http.NewRequest(http.MethodPut, tr.buildPath(fileName), strings.NewReader(byteString))
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: PUT,
			response: nil,
			message:  "Failed to initialize request for PutFile",
			err:      err,
			failed:   true,
		}
		return
	}

	response, err := tr.client.Do(req)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: PUT,
			response: response,
			message:  "Error executing http request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		fileName: fileName,
		testType: PUT,
		response: response,
		message:  responseToString(response),
		err:      err,
	}
}

func (tr *TestExecutor) CreateFile(fileName string) {
	tr.waitForOpenInProcess(fileName)
	defer func() {
		tr.inProcessLock.Lock()
		tr.inProcess.Delete(fileName)
		tr.inProcessLock.Unlock()
	}()
	fileSize := rand.Int63n(tr.maxFileSize) + 1
	fileBytes := make([]byte, fileSize)
	_, err := crand.Read(fileBytes)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CREATE,
			response: nil,
			message:  "Failed to generate random file bytes",
			err:      err,
			failed:   true,
		}
		return
	}

	byteString := b64.StdEncoding.EncodeToString(fileBytes)
	req, err := http.NewRequest(http.MethodPut, tr.buildPath(fileName), strings.NewReader(byteString))
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CREATE,
			response: nil,
			message:  "Failed to initialize request for PutFile",
			err:      err,
			failed:   true,
		}
		return
	}

	response, err := tr.client.Do(req)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CREATE,
			response: response,
			message:  "Error executing http request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		fileName: fileName,
		testType: CREATE,
		response: response,
		message:  responseToString(response),
		err:      err,
		failed:   response.StatusCode >= 400,
	}
}

func (tr *TestExecutor) GetFile(fileName string) {
	response, err := tr.client.Get(tr.buildPath(fileName))
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: GET,
			response: response,
			message:  "Error executing http GET request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		fileName: fileName,
		testType: GET,
		response: response,
		message:  responseToString(response),
		err:      err,
		failed:   response.StatusCode >= 400,
	}
}

func (tr *TestExecutor) DeleteFile(fileName string) {
	tr.waitForOpenInProcess(fileName)
	defer func() {
		tr.inProcessLock.Lock()
		tr.inProcess.Delete(fileName)
		tr.inProcessLock.Unlock()
	}()

	req, err := http.NewRequest(http.MethodDelete, tr.buildPath(fileName), nil)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: DELETE,
			response: nil,
			message:  "Failed ot build delete request.",
			err:      err,
			failed:   true,
		}
		return
	}

	response, err := tr.client.Do(req)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: DELETE,
			response: response,
			message:  "Error executing http DELETE request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		fileName: fileName,
		testType: DELETE,
		response: response,
		message:  responseToString(response),
		err:      err,
		failed:   response.StatusCode >= 400,
	}
}

func (tr *TestExecutor) ConsistencyCheck(fileName string) {
	tr.waitForOpenInProcess(fileName)
	defer func() {
		tr.inProcessLock.Lock()
		tr.inProcess.Delete(fileName)
		tr.inProcessLock.Unlock()
	}()

	fileSize := tr.randomFileSize()
	fileBytes := make([]byte, fileSize)
	_, err := crand.Read(fileBytes)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: nil,
			message:  "Failed to create file",
			err:      err,
			failed:   true,
		}
		return
	}

	// Perform write
	byteString := b64.StdEncoding.EncodeToString(fileBytes)
	req, err := http.NewRequest(http.MethodPut, tr.buildPath(fileName), strings.NewReader(byteString))
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: nil,
			message:  "Failed to initialize request for PutFile",
			err:      err,
			failed:   true,
		}
		return
	}

	response, err := tr.client.Do(req)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  "Error executing http request",
			err:      err,
			failed:   true,
		}
		return
	}

	if response.StatusCode != http.StatusCreated {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  fmt.Sprintf("PUT failed due to unexpected status code, got: %d but expected 201.", response.StatusCode),
			err:      err,
			failed:   true,
		}
		return
	}

	// Fetch immediately after write, verify data is consistent.
	response, err = tr.client.Get(tr.buildPath(fileName))
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  "Error executing http GET request",
			err:      err,
			failed:   true,
		}
		return
	}

	if response.StatusCode != http.StatusOK {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  fmt.Sprintf("GET failed due to unexpected status code, got: %d but expected 200.", response.StatusCode),
			err:      err,
			failed:   true,
		}
		return
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  fmt.Sprintf("Error decoding response body: %s", err.Error()),
			err:      err,
			failed:   true,
		}
		return
	}

	if string(body) != byteString {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  "Written and read body are not identical! Inconsistent data returned",
			err:      err,
			failed:   true,
		}
		return
	}

	req, err = http.NewRequest(http.MethodDelete, tr.buildPath(fileName), nil)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: nil,
			message:  "Failed to create delete request",
			err:      err,
			failed:   true,
		}
		return
	}

	response, err = tr.client.Do(req)
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  "Error executing http DELETE request",
			err:      err,
			failed:   true,
		}
		return
	}

	if response.StatusCode != http.StatusOK {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  fmt.Sprintf("DELETE failed due to unexpected status code, got: %d but expected 200.", response.StatusCode),
			err:      err,
			failed:   true,
		}
		return
	}

	response, err = tr.client.Get(tr.buildPath(fileName))
	if err != nil {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  fmt.Sprintf("Error performing GET for deleted file in consistent test. file: %s. Error: %s", fileName, err.Error()),
			err:      err,
			failed:   true,
		}
		return
	}

	if response.StatusCode != http.StatusNotFound {
		tr.results <- TestResult{
			fileName: fileName,
			testType: CONSISTENCY,
			response: response,
			message:  fmt.Sprintf("File was deleted but received non-404 http code on immediate get. Got: %d for file: %s", response.StatusCode, fileName),
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		fileName: fileName,
		testType: CONSISTENCY,
		response: response,
		message:  "Consistency check passed!",
		err:      nil,
		failed:   false,
	}
}

func (tr *TestExecutor) SetMaxFileSize(maxSize int64) {
	tr.fileSizeLock.Lock()
	defer tr.fileSizeLock.Unlock()
	tr.maxFileSize = maxSize
}

func (tr *TestExecutor) GetMaxFileSize() int64 {
	tr.fileSizeLock.RLock()
	defer tr.fileSizeLock.RUnlock()
	return tr.maxFileSize
}

// Returns a random file size that is less than the curren set maxFileSize
func (tr *TestExecutor) randomFileSize() int64 {
	tr.fileSizeLock.RLock()
	defer tr.fileSizeLock.RUnlock()
	return rand.Int63n(tr.maxFileSize) + 1
}

func (tr *TestExecutor) buildPath(fileName string) string {
	return fmt.Sprintf("%s://%s:%s/%s/%s", tr.endpointCfg.Proto, tr.endpointCfg.Host, tr.endpointCfg.Port, tr.endpointCfg.PathPrefix, fileName)
}

func responseToString(resp *http.Response) string {
	if resp != nil && resp.Body != nil {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return buf.String()
	}

	return ""
}
