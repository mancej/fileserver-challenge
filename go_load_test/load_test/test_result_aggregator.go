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
	startTime                          time.Time
	numRequests                        int
	numSuccess                         int
	numGet                             int
	numPut                             int
	numDelete                          int
	numConsistency                     int
	numFailure                         int
	numFailedConsistency               int
	numThrottled                       int
	intervalCount                      int
	interval                           time.Duration
	num500s                            int
	httpErrors                         []string
	otherErrors                        []string
	resultLock                         sync.RWMutex
	numLastInterval                    int
	numSuccessLastInterval             int
	numGetLastInterval                 int
	numPutLastInterval                 int
	numDeleteLastInterval              int
	avgGetDurationLastInterval         time.Duration
	avgPutDurationLastInterval         time.Duration
	avgDeleteDurationLastInterval      time.Duration
	avgConsistencyDurationLastInterval time.Duration
	numConsistencyLastInterval         int
	numThrottledLastInterval           int
	maxSeenSuccessfulRequestPerSec     int
	lastPrintedNumSuccess              int
	lastPrintedNumFailure              int
	lastPrintedNumRequests             int
	totalGetDuration                   time.Duration
	totalPutDuration                   time.Duration
	totalDeleteDuration                time.Duration
	totalConsistencyDuration           time.Duration
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
			msg := fmt.Sprintf("File: %s, Error: %s", result.FileName(), result.message)
			log.Error(msg)
			tr.httpErrors = append(tr.httpErrors, msg)
		} else if result.err != nil {
			msg := fmt.Sprintf("File: %s, Error: %s", result.FileName(), result.err.Error())
			log.Error(msg)
			tr.otherErrors = append(tr.otherErrors, msg)
		}
	}

	if result.WasTestFailure() && result.TestType() == CONSISTENCY {
		tr.numFailedConsistency++
		tr.otherErrors = append(tr.otherErrors, result.message)
	}

	// Increment items that are read by another goroutine with lock
	defer tr.resultLock.Unlock()
	tr.resultLock.Lock()

	tr.intervalCount++

	if result.testType == GET {
		tr.numGet++
		tr.totalGetDuration += result.duration
	} else if result.testType == PUT || result.testType == CREATE {
		tr.numPut++
		tr.totalPutDuration += result.duration
	} else if result.testType == DELETE {
		tr.numDelete++
		tr.totalDeleteDuration += result.duration
	} else if result.testType == CONSISTENCY {
		tr.totalConsistencyDuration += result.duration
		tr.numConsistency++
		tr.numRequests += 3
		tr.intervalCount += 3
		if result.WasSuccess() {
			tr.numSuccess += 3
		}
	}
}

func (tr *TestResults) PrintResults() {
	tr.resultLock.RLock()
	defer tr.resultLock.RUnlock()

	headerFmt := color.New(color.FgGreen, color.Underline).SprintfFunc()
	columnFmt := color.New(color.FgYellow).SprintfFunc()
	// Round to 1 decimal place
	currentThroughput := tr.numLastInterval
	currentSuccessful := tr.numSuccessLastInterval
	tbl := table.New("Metric", "Count", "", "")
	tbl.WithHeaderFormatter(headerFmt).WithFirstColumnFormatter(columnFmt)

	tbl.AddRow("# Requests", tr.numRequests, "", "")
	tbl.AddRow("# Test Success", tr.numSuccess, "", "")
	tbl.AddRow("# Test Failures", tr.numFailure, "")
	tbl.AddRow("# Consistency Test Success", tr.numConsistency-tr.numFailedConsistency, "")
	tbl.AddRow("# Consistency Test Failures", tr.numFailedConsistency, "")
	tbl.AddRow("# 5XX Errors", tr.num500s, "")
	tbl.AddRow("# Throttled", tr.numThrottled, "")
	tbl.AddRow("# Current THROTTLE/sec", tr.numThrottledLastInterval, "")
	tbl.AddRow("# Current GET/sec", tr.numGetLastInterval, "Avg Duration: ", tr.avgGetDurationLastInterval.Milliseconds())
	tbl.AddRow("# Current PUT/sec", tr.numPutLastInterval, "Avg Duration: ", tr.avgPutDurationLastInterval.Milliseconds())
	tbl.AddRow("# Current DELETE/sec", tr.numDeleteLastInterval, "Avg Duration: ", tr.avgDeleteDurationLastInterval.Milliseconds())
	tbl.AddRow("# Current CONSISTENCY/sec", tr.numConsistencyLastInterval, "(4 requests per check)", tr.avgConsistencyDurationLastInterval.Milliseconds())
	tbl.AddRow("Current req/sec", currentThroughput, "", "")
	tbl.AddRow("Current Successful req/sec", currentSuccessful, "", "")
	tbl.AddRow("Max Successful req/sec", tr.maxSeenSuccessfulRequestPerSec, "", "")
	tbl.Print()

	tr.lastPrintedNumFailure = tr.numFailure
	tr.lastPrintedNumSuccess = tr.numSuccess
	tr.lastPrintedNumRequests = tr.numRequests
}

func (tr *TestResults) PrintErrors() {
	tr.resultLock.RLock()
	defer tr.resultLock.RUnlock()

	fmt.Println()
	fmt.Println("HTTP Errors:")
	fmt.Println("---------------------------------------------")
	for i := 0; i < Min(len(tr.httpErrors), 5); i++ {
		fmt.Println(tr.httpErrors[len(tr.httpErrors)-i-1])
	}
	fmt.Println("")
	fmt.Println("Other Errors: ")
	fmt.Println("---------------------------------------------")
	for i := 0; i < Min(len(tr.otherErrors), 5); i++ {
		fmt.Println(tr.otherErrors[len(tr.otherErrors)-i-1])
	}
	fmt.Println()
	fmt.Println()
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
	go func() {
		var lastFiveIntervals, lastFiveIntervalsSuccess, lastFiveIntervalsGets,
			lastFiveIntervalsPuts, lastFiveIntervalsDeletes, lastFiveIntervalsThrottles,
			lastFiveIntervalsConsistency []int

		var lastFiveIntervalsConsistencyDuration, lastFiveIntervalsGetDuration, lastFiveIntervalsPutDuration,
			lastFiveIntervalsDeleteDuration []time.Duration
		var totalSuccessLastInterval, totalGetLastInterval, totalPutLastInterval,
			totalDeleteLastInterval, totalThrottlesLastInterval, totalConsistencyLastInterval int

		var totalGetDurationLastInterval, totalPutDurationLastInterval, totalDeleteDurationLastInterval,
			totalConsistencyDurationLastInterval time.Duration
		lastUpdate := time.Now()

		for {
			time.Sleep(time.Millisecond * 50)
			if time.Now().Sub(lastUpdate) > ra.Results.interval {
				lastFiveIntervals = append(lastFiveIntervals, ra.Results.intervalCount)
				lastFiveIntervalsSuccess = append(lastFiveIntervalsSuccess, ra.Results.numSuccess-totalSuccessLastInterval)
				lastFiveIntervalsGets = append(lastFiveIntervalsGets, ra.Results.numGet-totalGetLastInterval)
				lastFiveIntervalsPuts = append(lastFiveIntervalsPuts, ra.Results.numPut-totalPutLastInterval)
				lastFiveIntervalsDeletes = append(lastFiveIntervalsDeletes, ra.Results.numDelete-totalDeleteLastInterval)
				lastFiveIntervalsThrottles = append(lastFiveIntervalsThrottles, ra.Results.numThrottled-totalThrottlesLastInterval)
				lastFiveIntervalsConsistency = append(lastFiveIntervalsConsistency, ra.Results.numConsistency-totalConsistencyLastInterval)
				lastFiveIntervalsGetDuration = append(lastFiveIntervalsGetDuration, getIntervalAvgDuration(ra.Results.totalGetDuration, totalGetDurationLastInterval, ra.Results.numGetLastInterval))
				lastFiveIntervalsPutDuration = append(lastFiveIntervalsPutDuration, getIntervalAvgDuration(ra.Results.totalPutDuration, totalPutDurationLastInterval, ra.Results.numPutLastInterval))
				lastFiveIntervalsDeleteDuration = append(lastFiveIntervalsDeleteDuration, getIntervalAvgDuration(ra.Results.totalDeleteDuration, totalDeleteDurationLastInterval, ra.Results.numDeleteLastInterval))
				lastFiveIntervalsConsistencyDuration = append(lastFiveIntervalsConsistencyDuration, getIntervalAvgDuration(ra.Results.totalConsistencyDuration, totalConsistencyDurationLastInterval, ra.Results.numConsistencyLastInterval))
				totalSuccessLastInterval = ra.Results.numSuccess
				totalGetLastInterval = ra.Results.numGet
				totalPutLastInterval = ra.Results.numPut
				totalDeleteLastInterval = ra.Results.numDelete
				totalThrottlesLastInterval = ra.Results.numThrottled
				totalConsistencyLastInterval = ra.Results.numConsistency
				totalGetDurationLastInterval = ra.Results.totalGetDuration
				totalPutDurationLastInterval = ra.Results.totalPutDuration
				totalDeleteDurationLastInterval = ra.Results.totalDeleteDuration
				totalConsistencyDurationLastInterval = ra.Results.totalConsistencyDuration

				if len(lastFiveIntervalsSuccess) > 4 {
					lastFiveIntervalsSuccess = lastFiveIntervalsSuccess[1:]
					lastFiveIntervals = lastFiveIntervals[1:]
					lastFiveIntervalsGets = lastFiveIntervalsGets[1:]
					lastFiveIntervalsPuts = lastFiveIntervalsPuts[1:]
					lastFiveIntervalsDeletes = lastFiveIntervalsDeletes[1:]
					lastFiveIntervalsThrottles = lastFiveIntervalsThrottles[1:]
					lastFiveIntervalsConsistency = lastFiveIntervalsConsistency[1:]
					lastFiveIntervalsGetDuration = lastFiveIntervalsGetDuration[1:]
					lastFiveIntervalsPutDuration = lastFiveIntervalsPutDuration[1:]
					lastFiveIntervalsDeleteDuration = lastFiveIntervalsDeleteDuration[1:]
					lastFiveIntervalsConsistencyDuration = lastFiveIntervalsConsistencyDuration[1:]
				}

				ra.Results.resultLock.Lock()
				lastUpdate = time.Now()
				ra.Results.numLastInterval = average(lastFiveIntervals)
				ra.Results.numSuccessLastInterval = average(lastFiveIntervalsSuccess)
				ra.Results.numGetLastInterval = average(lastFiveIntervalsGets)
				ra.Results.numPutLastInterval = average(lastFiveIntervalsPuts)
				ra.Results.numDeleteLastInterval = average(lastFiveIntervalsDeletes)
				ra.Results.numThrottledLastInterval = average(lastFiveIntervalsThrottles)
				ra.Results.numConsistencyLastInterval = average(lastFiveIntervalsConsistency)
				ra.Results.avgGetDurationLastInterval = avgDuration(lastFiveIntervalsGetDuration)
				ra.Results.avgPutDurationLastInterval = avgDuration(lastFiveIntervalsPutDuration)
				ra.Results.avgDeleteDurationLastInterval = avgDuration(lastFiveIntervalsDeleteDuration)
				ra.Results.avgConsistencyDurationLastInterval = avgDuration(lastFiveIntervalsConsistencyDuration)
				ra.Results.intervalCount = 0
				if ra.Results.numSuccessLastInterval > ra.Results.maxSeenSuccessfulRequestPerSec {
					ra.Results.maxSeenSuccessfulRequestPerSec = ra.Results.numSuccessLastInterval
				}
				ra.Results.resultLock.Unlock()

				if ra.Results.numFailure > MaxFailuresBeforeExit {
					close(ra.cfg.ShutdownChan)
					break
				}

			}
		}
	}()

	keepRunning := true
	for keepRunning {
		var testResult TestResult
		testResult, keepRunning = <-ra.resultsChan
		ra.Results.Merge(testResult)
		if (testResult.WasTestFailure() || testResult.Was404()) && keepRunning {
			ra.cfg.FailureChan <- testResult
		}

		if testResult.WasSuccess() && keepRunning {
			ra.cfg.SuccessChan <- testResult
		}
	}

}

func (ra *ResultAggregator) PrintScore() {
	scoreModifier := time.Now().Sub(ra.Results.startTime).Minutes() // longer running = better.

	consistencyRate := float64(1) - float64(ra.Results.numFailedConsistency)/float64(ra.Results.numConsistency)
	successRate := float64(1) - float64(ra.Results.numFailure)/float64(ra.Results.numSuccess+ra.Results.numFailure)
	fmt.Printf("Your consistency accuracy was %f percent", math.Round(consistencyRate*10000)/10000*100)
	fmt.Println()
	fmt.Printf("Your success rate was %f percent", math.Round(successRate*10000)/10000*100)
	fmt.Println()
	fmt.Printf("Your maximum achieved successful requests/sec was %d", ra.Results.maxSeenSuccessfulRequestPerSec)
	fmt.Println()
	fmt.Printf("Your test completed after %d seconds.", int(time.Now().Sub(ra.Results.startTime).Seconds()))
	fmt.Println()
	score := int(math.Round(float64(ra.Results.maxSeenSuccessfulRequestPerSec) * scoreModifier * consistencyRate * successRate))
	fmt.Printf("Your total score is: %d.", score)
	fmt.Println()
}

func average(items []int) int {
	sum := 0
	for i := 0; i < len(items); i++ {
		sum = sum + items[i]
	}

	return sum / len(items)
}

func avgDuration(items []time.Duration) time.Duration {
	var sum int64
	for i := 0; i < len(items); i++ {
		sum = sum + items[i].Nanoseconds()
	}

	return time.Duration(sum / int64(len(items)))
}

func getIntervalAvgDuration(totalDuration, totalDurationLastInterval time.Duration, totalRequestsOfTypeLastInterval int) time.Duration {
	intervalDur := totalDuration - totalDurationLastInterval

	if totalRequestsOfTypeLastInterval == 0 {
		totalRequestsOfTypeLastInterval = 1
	}

	return time.Duration(intervalDur.Nanoseconds() / int64(totalRequestsOfTypeLastInterval))
}
