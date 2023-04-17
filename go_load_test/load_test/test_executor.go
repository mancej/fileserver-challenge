package load_test

import (
	"bytes"
	crand "crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type FileSet map[string]bool

func (s FileSet) Has(item string) bool {
	_, ok := s[item]
	return ok
}

func (s FileSet) Delete(item string) {
	delete(s, item)
}

func (s FileSet) Add(item string) {
	s[item] = true
}

func (s FileSet) RandomFile() string {
	length := len(s)
	if length == 0 {
		return ""
	}

	keys := make([]string, length)

	i := 0
	for k := range s {
		keys[i] = k
		i++
	}

	randomIdx := rand.Intn(length)
	return keys[randomIdx]
}

type TestExecutor struct {
	client        *http.Client
	inProcess     FileSet
	maxFileSize   int64
	inProcessLock sync.RWMutex
	results       chan TestResult
	endpointCfg   TestEndpointConfig
}

type TestFunc func(fileName string)

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
	fileSize := rand.Int63n(tr.maxFileSize)
	fileBytes := make([]byte, fileSize)
	_, err := crand.Read(fileBytes)
	if err != nil {
		tr.results <- TestResult{
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
			response: response,
			message:  "Error executing http request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		response: response,
		message:  responseToString(response),
		err:      err,
	}
}

func (tr *TestExecutor) GetFile(fileName string) {
	response, err := tr.client.Get(tr.buildPath(fileName))
	if err != nil {
		tr.results <- TestResult{
			response: response,
			message:  "Error executing http GET request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		response: response,
		message:  responseToString(response),
		err:      err,
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
			response: response,
			message:  "Error executing http DELETE request",
			err:      err,
			failed:   true,
		}
		return
	}

	tr.results <- TestResult{
		response: response,
		message:  responseToString(response),
		err:      err,
	}
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
