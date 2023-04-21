package load_test

import (
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

// Run Listens to scheduled test chann and runs tests
func (tr *TestRunner) Run() {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        500,
			MaxIdleConnsPerHost: 100,
			MaxConnsPerHost:     0,
			IdleConnTimeout:     0,
		},
		Timeout: time.Second * 2,
	}
	exec := NewTestExecutor(client, tr.cfg.EndpointCfg, tr.cfg.TestConfig, tr.cfg.ResultChan)

	keepRunning := true
	var test Test
	for keepRunning {
		var funcToRun func()
		test, keepRunning = <-tr.cfg.ScheduleChan

		if !keepRunning {
			break
		}

		if test.TestType == GET {
			funcToRun = func() {
				exec.GetFile(test.fileName)
			}
		} else if test.TestType == PUT {
			funcToRun = func() {
				exec.PutFile(test.fileName)
			}
		} else if test.TestType == DELETE {
			funcToRun = func() {
				exec.DeleteFile(test.fileName)
			}
		} else if test.TestType == CREATE {
			funcToRun = func() {
				exec.CreateFile(test.fileName)
			}
		} else if test.TestType == CONSISTENCY {
			funcToRun = func() {
				exec.ConsistencyCheck(test.fileName)
			}
		}

		go funcToRun()
	}
}
