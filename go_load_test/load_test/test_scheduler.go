package load_test

import (
	log "github.com/sirupsen/logrus"
	"math/rand"
	"sync"
	"time"
)

// Schedules tests (by pushing them to a channel)  at a particular cadence based on configurations.

type TestType string

const (
	GET         TestType = "GET"
	PUT         TestType = "PUT"
	DELETE      TestType = "DELETE"
	CREATE      TestType = "CREATE"
	CONSISTENCY TestType = "CONSISTENCY"
)

type Test struct {
	TestType
	fileName string
}

type TestCadenceConfig struct {
	Duration         time.Duration
	TestsPerDuration int
}

type TestConfig struct {
	MaxFileSize  int64
	MaxFileCount int
}

type TestSchedulerConfig struct {
	EndpointCfg      TestEndpointConfig
	SeedCadence      TestCadenceConfig
	SeedGrowthAmount int
	TestConfig       TestConfig
	SchedulerChan    chan Test
	ResultChan       chan TestResult
	FailureChan      chan TestResult // All test failures are published here.
	ShutdownChan     chan bool
}

type TestScheduler struct {
	cfg             TestSchedulerConfig
	scheduleLock    sync.Mutex
	seedResetTime   time.Time
	numScheduled    int
	totalScheduled  int64
	growthFactor    int // each time growth cadence is met, growth factor increases by 1. Total growth = growth config * growth factor
	tests           []TestType
	trackedFiles    FileSet
	trackedFileLock sync.RWMutex
	startTime       time.Time
}

// NewTestScheduler - Tests are immediately scheduled at the seed cadence, and will grow at a rate of seed + repeating growth cadence.
// I.E if seed is 5 req/s and growth is 1 req/sec, tests will schedule at 5/sec, then 1 sec later, 6/sec, then
// one sec later, 7/sec, etc.
func NewTestScheduler(cfg TestSchedulerConfig) TestScheduler {
	tests := []TestType{PUT, PUT, DELETE}
	for i := 0; i < 75; i++ {
		tests = append(tests, GET)
	}

	return TestScheduler{
		cfg:          cfg,
		growthFactor: 0,
		tests:        tests,
		trackedFiles: make(FileSet),
		startTime:    time.Now(),
	}
}

func (ts *TestScheduler) Run() {
	keepRunning := true
	ts.seedResetTime = time.Now().Add(ts.cfg.SeedCadence.Duration)

	go ts.MergeFailedTestResults()

	for keepRunning {
		ts.ScheduleTests()
		select {
		case _, keepRunning = <-ts.cfg.ShutdownChan:
		default:
		}
		time.Sleep(time.Microsecond * 50)
	}
}

// ScheduleTests schedules tests on the channel if we haven't met our quota based on seed configs
func (ts *TestScheduler) ScheduleTests() {
	targetSeed := ts.cfg.SeedCadence.TestsPerDuration + (ts.growthFactor * ts.cfg.SeedGrowthAmount)
	startTime := time.Now()
	for ts.numScheduled < targetSeed {
		ts.cfg.SchedulerChan <- ts.GetTestFunc()
		ts.numScheduled++
		ts.totalScheduled++
		remainingTime := ts.cfg.SeedCadence.Duration - time.Now().Sub(startTime) // remaining time before reset

		// Spaces out scheduling of requests over the duration the seed duration so we don't
		// schedule + run all N requests instantly. This ensures a smooth rate of scheduled / executed tests.
		seedsLeft := targetSeed - ts.numScheduled
		if seedsLeft > 0 {
			time.Sleep(remainingTime / time.Duration(seedsLeft))
		}
	}

	// If we are after our reset time, reset to a new time, and reset num scheduled to whatever's left, or 0
	if time.Now().UnixMicro() > ts.seedResetTime.UnixMicro() {
		ts.seedResetTime = time.Now().Add(ts.cfg.SeedCadence.Duration)
		ts.numScheduled = targetSeed - ts.numScheduled
		ts.growthFactor++
		log.Infof("Now scheduling: %d req/sec", ts.cfg.SeedCadence.TestsPerDuration+(ts.growthFactor*ts.cfg.SeedGrowthAmount))
		log.Infof("Num tracked files: %d", len(ts.trackedFiles))
	}

}

// TrackedFiles assumes all reads/writes/deletes were success. It doesn't add file back if delete was failure, etc.

// GetTestFunc selects a psuedo random test function to run
func (ts *TestScheduler) GetTestFunc() Test {
	rand.Seed(time.Now().UnixNano())
	createNewFile := rand.Intn(ts.cfg.TestConfig.MaxFileCount) > len(ts.trackedFiles)
	var testToRun = Test{}

	ts.trackedFileLock.RLock()
	defer ts.trackedFileLock.RUnlock()
	if createNewFile {
		testToRun.fileName = RandStringBytes(15)
		// Give 2% chance to execute consistency test, or a higher % chance the more tracked files there are
		// If the load test just started, only run consistency tests for the first 15 seconds.
		bonus := ts.cfg.TestConfig.MaxFileCount / (ts.cfg.TestConfig.MaxFileCount - len(ts.trackedFiles) + 1)
		runConsistencyTest := rand.Intn(100)+bonus >= 98 || time.Now().Sub(ts.startTime) < time.Second*15
		if runConsistencyTest {
			// This tests is 4 requests total, so add 3 extra.
			ts.numScheduled += 3
			testToRun.TestType = CONSISTENCY
		} else {
			testToRun.TestType = CREATE
			ts.trackedFiles.Add(testToRun.fileName)
		}
	} else {
		testId := rand.Intn(len(ts.tests))
		testToRun.fileName = ts.trackedFiles.RandomFile()
		testToRun.TestType = ts.tests[testId]
		if testToRun.TestType == DELETE {
			ts.trackedFiles.Delete(testToRun.fileName)
		}
	}

	log.Debugf("Performing %s on file: %s", testToRun.TestType, testToRun.fileName)
	return testToRun
}

// MergeFailedTestResults listens to failed tests and updates trackedFiles based on results.
func (ts *TestScheduler) MergeFailedTestResults() {
	// Cleanup tracked files that were write / delete failures
	for {
		result, hasMore := <-ts.cfg.FailureChan
		if !hasMore {
			break
		}

		ts.trackedFileLock.Lock()
		if result.WasTestFailure() {
			if result.TestType() == DELETE {
				ts.trackedFiles.Add(result.FileName())
			}

			if result.TestType() == CREATE {
				ts.trackedFiles.Delete(result.FileName())
			}

			if result.TestType() == GET && result.Was404() {
				ts.trackedFiles.Delete(result.FileName())
			}

			if result.TestType() == CONSISTENCY {
				ts.trackedFiles.Delete(result.FileName())
			}
		}
		ts.trackedFileLock.Unlock()
	}
}
