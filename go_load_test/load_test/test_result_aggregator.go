package load_test

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/rodaine/table"
	log "github.com/sirupsen/logrus"
	"math"
	"sync"
	"time"
)

// Listens to a channel of test results. Aggregates results + provides metrics.

type TestResults struct {
	startTime       time.Time
	numRequests     int
	numSuccess      int
	numFailure      int
	numThrottled    int
	intervalCount   int
	interval        time.Duration
	num500s         int
	httpErrors      []string
	otherErrors     []string
	resultLock      sync.RWMutex
	numLastInterval int
}

func (tr *TestResults) Merge(result TestResult) {
	tr.numRequests++

	if result.WasSuccess() {
		tr.numSuccess++
	}

	if result.WasTestFailure() {
		tr.numFailure++
	}

	if result.Was5XX() {
		tr.num500s++
	}

	if result.WasThrottled() {
		tr.numThrottled++
	}

	if result.WasError() {
		if result.response != nil {
			tr.httpErrors = append(tr.httpErrors, result.message)
		}

		if result.err != nil {
			tr.otherErrors = append(tr.otherErrors, result.err.Error())
		}
	}

	defer tr.resultLock.Unlock()
	tr.resultLock.Lock()
	tr.intervalCount++
}

func (tr *TestResults) PrintResults() {
	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()

	// Round to 1 decimal place
	throughput := math.Round(float64(tr.numRequests)/time.Now().Sub(tr.startTime).Seconds()*10) / 10
	currentThroughput := tr.numLastInterval
	successThroughput := math.Round(float64(tr.numSuccess)/time.Now().Sub(tr.startTime).Seconds()*10) / 10
	tbl := table.New("Metric", "Count", "")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	tbl.AddRow("# Requests", tr.numRequests, "")
	tbl.AddRow("# Success", tr.numSuccess, "")
	tbl.AddRow("Current Throughput", currentThroughput, "")
	tbl.AddRow("Average Throughput", throughput, "")
	tbl.AddRow("Successful Throughput", successThroughput, "")
	tbl.AddRow("# Failures", tr.numFailure)
	tbl.AddRow("# 5XX Errors", tr.num500s)
	tbl.AddRow("# Throttled", tr.numThrottled)
	tbl.Print()
}

func (tr *TestResults) PrintErrors() {
	fmt.Println()
	fmt.Println("HTTP Errors:")
	fmt.Println("---------------------------------------------")
	for i := 0; i < Min(len(tr.httpErrors), 5); i++ {
		fmt.Print(tr.httpErrors[len(tr.httpErrors)-i-1])
	}
	fmt.Println("")
	fmt.Println("Other Errors: ")
	fmt.Println("---------------------------------------------")
	for i := 0; i < Min(len(tr.otherErrors), 5); i++ {
		fmt.Println(tr.otherErrors[len(tr.otherErrors)-i-1])
	}
}

type ResultAggregator struct {
	resultsChan chan TestResult
	cfg         TestSchedulerConfig
	Results     *TestResults
}

func NewResultAggregator(cfg TestSchedulerConfig) *ResultAggregator {
	return &ResultAggregator{
		resultsChan: cfg.ResultChan,
		cfg:         cfg,
		Results: &TestResults{
			startTime: time.Now(),
			interval:  cfg.SeedCadence.Duration,
		},
	}
}

func (ra *ResultAggregator) Run() {
	keepRunning := true
	go func() {
		var lastFiveIntervals []int
		lastUpdate := time.Now()
		for {

			time.Sleep(time.Millisecond * 25)
			if time.Now().Sub(lastUpdate) > ra.Results.interval {
				lastFiveIntervals = append(lastFiveIntervals, ra.Results.intervalCount)
				// Keep last 5 intervals for current throughput calculation
				if len(lastFiveIntervals) > 4 {
					lastFiveIntervals = lastFiveIntervals[1:]
				}

				ra.Results.resultLock.Lock()
				lastUpdate = time.Now()
				ra.Results.numLastInterval = average(lastFiveIntervals)
				ra.Results.intervalCount = 0
				ra.Results.resultLock.Unlock()
				log.Infof("Channel length: %d", len(ra.resultsChan))
				log.Infof("Schedule length: %d", len(ra.cfg.SchedulerChan))
			}
		}
	}()

	for keepRunning {
		var testResult TestResult
		testResult, keepRunning = <-ra.resultsChan
		ra.Results.Merge(testResult)
	}
}

func average(items []int) int {
	sum := 0
	for i := 0; i < len(items); i++ {
		sum = sum + items[i]
	}

	log.Info("SUM: %d", sum)
	log.Info("AVERGAE: %d", sum/len(items))
	return sum / len(items)
}
