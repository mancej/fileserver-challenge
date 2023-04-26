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
	MaxFileSize           int64
	MaxFileCount          int
	FileSizeRamp          bool
	UploadRandomLargeFile bool
	MaxWritesPerCadence   int
}

type TestSchedulerConfig struct {
	EndpointCfg       TestEndpointConfig
	SeedCadence       TestCadenceConfig
	SeedGrowthAmount  float64
	EnableRequestRamp bool
	TestConfig        TestConfig
	SchedulerChan     chan Test
	ResultChan        chan TestResult
	FailureChan       chan TestResult // All test failures are published here.
	SuccessChan       chan TestResult // All test successes published here.
	ShutdownChan      chan bool
}

type TestScheduler struct {
	cfg            TestSchedulerConfig
	seedResetTime  time.Time
	numScheduled   int
	totalScheduled int64
	growthFactor   int // each time growth cadence is met, growth factor increases by 1. Total growth = growth config * growth factor
	tests          []TestType
	trackedFiles   FileSet

	trackedFileLock sync.RWMutex
	startTime       time.Time
	rampAmount      int
	rampFactor      int
	lastRamp        time.Time
}

// NewTestScheduler - Tests are immediately scheduled at the seed cadence, and will grow at a rate of seed + repeating growth cadence.
// I.E if seed is 5 req/s and growth is 1 req/sec, tests will schedule at 5/sec, then 1 sec later, 6/sec, then
// one sec later, 7/sec, etc.
func NewTestScheduler(cfg TestSchedulerConfig) TestScheduler {
	tests := []TestType{PUT, DELETE}
	for i := 0; i < 75; i++ {
		tests = append(tests, GET)
	}

	return TestScheduler{
		cfg:          cfg,
		growthFactor: 0,
		tests:        tests,
		trackedFiles: make(FileSet),
		startTime:    time.Now(),
		rampFactor:   1,
		rampAmount:   0,
		lastRamp:     time.Now(),
	}
}

func (ts *TestScheduler) Run() {
	keepRunning := true
	ts.seedResetTime = time.Now().Add(ts.cfg.SeedCadence.Duration)
	go ts.MergeFailedTestResults()
	go ts.MergeSuccessfulTestResults()

	for keepRunning {
		// Schedule tests.
		ts.ScheduleTests()

		select {
		case _, keepRunning = <-ts.cfg.ShutdownChan:
		default:
		}
		time.Sleep(time.Microsecond * 50)
	}

	close(ts.cfg.SchedulerChan)
}

// ScheduleTests schedules tests on the channel if we haven't met our quota based on seed configs
func (ts *TestScheduler) ScheduleTests() {
	targetSeed := ts.cfg.SeedCadence.TestsPerDuration + int(float64(ts.growthFactor)*float64(ts.cfg.SeedGrowthAmount)) + ts.rampAmount
	seedCount := targetSeed // num in this seed that need to be scheduled.
	startTime := time.Now()
	numWrites := 0

	for ts.numScheduled < seedCount {
		scheduleStart := time.Now()
		test := ts.GetTestFunc(numWrites < ts.cfg.TestConfig.MaxWritesPerCadence)
		ts.numScheduled++
		ts.totalScheduled++

		// If writes exceed
		if test.TestType == CREATE || test.TestType == PUT {
			numWrites++
		}

		ts.cfg.SchedulerChan <- test

		remainingTime := ts.cfg.SeedCadence.Duration - time.Now().Sub(startTime) // remaining time before reset

		// Calculates time it takes to schedule a job.
		scheduleDur := time.Now().Sub(scheduleStart)

		// Spaces out scheduling of requests over the duration the seed duration so we don't
		// schedule + run all N requests instantly. This ensures a smooth rate of scheduled / executed tests.
		// also removes a flat 50 microseconds to account for time for this calculation and provide a buffer
		seedsLeft := seedCount - ts.numScheduled
		if seedsLeft > 0 {
			time.Sleep((remainingTime / time.Duration(seedsLeft)) - scheduleDur - time.Microsecond*50)
		}
	}

	// If we are after our reset time, reset to a new time, and reset num scheduled to whatever's left, or 0
	if time.Now().UnixMicro() > ts.seedResetTime.UnixMicro() {
		ts.seedResetTime = time.Now().Add(ts.cfg.SeedCadence.Duration)
		ts.numScheduled = seedCount - ts.numScheduled
		ts.growthFactor++

		// If request ramp is eanbled, ramp requests rates
		if ts.cfg.EnableRequestRamp {
			if time.Now().Sub(ts.lastRamp) > time.Minute {
				ts.rampFactor++
				ts.lastRamp = time.Now()
			}
			ts.rampAmount = ts.rampAmount + int(float64(ts.cfg.SeedGrowthAmount)*float64(ts.rampFactor))
		}

		log.Infof("Now scheduling: %d req/sec", targetSeed)
		log.Infof("Schedule Chan length: %d", len(ts.cfg.SchedulerChan))
		log.Infof("Result Chan Length: %d", len(ts.cfg.ResultChan))
	}

}

// TrackedFiles assumes all reads/writes/deletes were success. It doesn't add file back if delete was failure, etc.

// GetTestFunc selects a psuedo random test function to run
// If canBeWrite is false, then only a DELETE or GET will be scheduled.
func (ts *TestScheduler) GetTestFunc(canBeWrite bool) Test {
	//rand.Seed(time.Now().UnixNano())
	createNewFile := canBeWrite && rand.Intn(ts.cfg.TestConfig.MaxFileCount) > len(ts.trackedFiles)
	var testToRun = Test{}

	if createNewFile {
		testToRun.fileName = RandStringBytes(15)
		// Give 2% chance to execute consistency test, or a higher % chance the more tracked files there are
		// If the load test just started, only run consistency tests for the first 5 seconds.
		bonus := Min(ts.cfg.TestConfig.MaxFileCount/(ts.cfg.TestConfig.MaxFileCount-len(ts.trackedFiles)+1), 8)
		runConsistencyTest := rand.Intn(100)+bonus >= 98 || time.Now().Sub(ts.startTime) < time.Second*5
		if runConsistencyTest {
			// This tests is 4 requests total, so add 3 extra.
			ts.numScheduled += 3
			testToRun.TestType = CONSISTENCY
			log.Infof("Scheduling consistency test for file: %s", testToRun.fileName)
		} else {
			// only add to tracked after a success is returned on success chan. This prevents a quickly scheduled GET from
			// failing with 404 after a 429 was returned on CREATE
			testToRun.TestType = CREATE
		}
	} else {
		testId := rand.Intn(len(ts.tests))
		ts.trackedFileLock.RLock()
		testToRun.fileName = ts.trackedFiles.RandomFile()
		ts.trackedFileLock.RUnlock()
		testToRun.TestType = ts.tests[testId]
		if testToRun.TestType == DELETE {
			ts.trackedFileLock.Lock()
			ts.trackedFiles.Delete(testToRun.fileName)
			ts.trackedFileLock.Unlock()
		}

		if !canBeWrite && testToRun.TestType == PUT {
			testToRun.TestType = GET
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

// MergeSuccessfulTestResults listens to failed tests and updates trackedFiles based on results.
func (ts *TestScheduler) MergeSuccessfulTestResults() {
	// Cleanup tracked files that were write / delete failures
	for {
		result, hasMore := <-ts.cfg.SuccessChan
		if !hasMore {
			break
		}

		ts.trackedFileLock.Lock()
		if result.TestType() == CREATE {
			ts.trackedFiles.Add(result.FileName())
		}
		ts.trackedFileLock.Unlock()
	}
}
