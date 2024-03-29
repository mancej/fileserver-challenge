package load_test

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

// Listens to a channel of requested tests + runs them

func NewTestRunner(cfg TestRunnerConfig) *TestRunner {
	return &TestRunner{
		cfg: cfg}
}

type TestRunner struct {
	cfg TestRunnerConfig
}

type TestRunnerConfig struct {
	TestConfig
	EndpointCfg  TestEndpointConfig
	ResultChan   chan TestResult
	ScheduleChan chan Test
}

// Run Listens to scheduler test chan and runs tests
func (tr *TestRunner) Run() {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:    45000,
			MaxConnsPerHost: 0,
		},
		Timeout: time.Second * 20,
	}
	exec := NewTestExecutor(client, tr.cfg.EndpointCfg, tr.cfg.TestConfig, tr.cfg.ResultChan)

	lastFileSizeUpdate := time.Now()

	// Ramp maximum file size for writes
	go func() {
		if tr.cfg.FileSizeRamp {
			for {
				if time.Now().Sub(lastFileSizeUpdate) > time.Second*15 {
					// Increase file size by 50% every 15 seconds
					fileSize := int64(float64(exec.GetMaxFileSize()) * 1.5)
					exec.SetMaxFileSize(fileSize)
					log.Infof("Increasing max file size due to ramp. New max size is: %d bytes", fileSize)
					lastFileSizeUpdate = time.Now()
				}
				time.Sleep(time.Second)
			}
		}
	}()

	keepRunning := true
	var test Test
	for keepRunning {
		var funcToRun func()
		test, keepRunning = <-tr.cfg.ScheduleChan
		if !keepRunning {
			break
		}

		switch test.TestType {
		case GET:
			funcToRun = func() {
				exec.GetFile(test.fileName)
			}
		case PUT:
			funcToRun = func() {
				exec.PutFile(test.fileName)
			}
		case DELETE:
			funcToRun = func() {
				exec.DeleteFile(test.fileName)
			}
		case CREATE:
			funcToRun = func() {
				exec.CreateFile(test.fileName)
			}
		case CONSISTENCY:
			funcToRun = func() {
				exec.ConsistencyCheck(test.fileName)
			}
		default:
			funcToRun = func() {
				exec.GetFile(test.fileName)
			}
		}

		go funcToRun()
	}
}
