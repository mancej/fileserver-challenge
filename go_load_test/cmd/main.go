package main

import (
	"fmt"
	"github.com/mancej/fileserver-challenge/go_load_test/load_test"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// Design plans
// 1 While loop that shovels x req/sec into queue. The item passed into queue is refernece to method to run.
// 1 Queue consumer that reads + spawns goroutines for reach request
// N Goroutines that run request + report results back via queue
// 1 Result aggregator that reads results and publishes them.

func main() {
	start := time.Now()
	load_test.InitClear()
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	file, err := os.OpenFile("/tmp/load_test.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic("Cannot create log file. Does your system have permissions to create a file at /tmp/?")
	}

	log.SetOutput(file)

	host := load_test.GetEnv("FILE_SERVER_HOST", "localhost")
	port := load_test.GetEnv("FILE_SERVER_PORT", "1234")
	proto := load_test.GetEnv("FILE_SERVER_PROTO", "http")
	prefix := load_test.GetEnv("FILE_SERVER_PATH_PREFIX", "api/fileserver")
	maxFileCount, _ := strconv.Atoi(load_test.GetEnv("MAX_FILE_COUNT", "500"))
	maxFileSize, _ := strconv.ParseInt(load_test.GetEnv("MAX_FILE_SIZE", "1024"), 10, 64)
	requestsPerSecond, _ := strconv.Atoi(load_test.GetEnv("REQUESTS_PER_SECOND", "1"))
	seedGrowthAmount, _ := strconv.Atoi(load_test.GetEnv("SEED_GROWTH_AMOUNT", "1"))

	cfg := load_test.TestSchedulerConfig{
		EndpointCfg: load_test.TestEndpointConfig{
			Proto:      proto,
			Host:       host,
			Port:       port,
			PathPrefix: prefix,
		},
		SeedCadence: load_test.TestCadenceConfig{
			Duration:         time.Second,
			TestsPerDuration: requestsPerSecond,
		},
		SeedGrowthAmount: seedGrowthAmount,
		TestConfig: load_test.TestConfig{
			MaxFileSize:  maxFileSize,
			MaxFileCount: maxFileCount,
		},
		SchedulerChan: make(chan load_test.Test, 50000),       // Tests scheduled to run asap are sent here
		ResultChan:    make(chan load_test.TestResult, 15000), // Results of tests are sent here
		ShutdownChan:  make(chan bool, 1),                     // If closed, shuts down scheduling
	}

	testRunnerCfg := load_test.TestRunnerConfig{
		TestConfig:   cfg.TestConfig,
		EndpointCfg:  cfg.EndpointCfg,
		ResultChan:   cfg.ResultChan,
		ScheduleChan: cfg.SchedulerChan,
	}

	log.Infof("Starting Scheduler.")
	scheduler := load_test.NewTestScheduler(cfg)
	go scheduler.Run()

	log.Info("Starting Runner.")
	runner := load_test.NewTestRunner(testRunnerCfg)
	go runner.Run()

	log.Info("Starting Result Aggregator")
	aggregator := load_test.NewResultAggregator(cfg)
	go aggregator.Run()

	// Repeatedly print results
	go func() {
		for {
			time.Sleep(time.Second)
			load_test.CallClear()
			aggregator.Results.PrintResults()
			aggregator.Results.PrintErrors()
		}
	}()

	// Wait for ctrl +c
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	keepRunning := true
	go func() {
		<-c
		fmt.Println("\r- Ctrl+C pressed in Terminal")
		close(cfg.ResultChan)
		close(cfg.ShutdownChan)
		close(cfg.ResultChan)
		keepRunning = false
	}()

	for keepRunning {
		time.Sleep(time.Second)
	}

	finish := time.Now()
	totalTime := finish.Sub(start)
	log.Infof("Finished in %f seconds.", totalTime.Seconds())
}
