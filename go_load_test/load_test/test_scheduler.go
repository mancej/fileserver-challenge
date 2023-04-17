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
	GET    TestType = "GET"
	PUT    TestType = "PUT"
	DELETE TestType = "DELETE"
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
	ShutdownChan     chan bool
}

type TestScheduler struct {
	seed             TestCadenceConfig
	seedGrowthAmount int
	scheduleChan     chan Test
	shutdownChan     chan bool // if this channel is closed, tests will stop being scheduled
	scheduleLock     sync.Mutex
	seedResetTime    time.Time
	numScheduled     int
	totalScheduled   int64
	growthFactor     int // each time growth cadence is met, growth factor increases by 1. Total groth = growth config * growth factor
	tests            []TestType
	trackedFiles     FileSet
	testConfig       TestConfig
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
		seed:             cfg.SeedCadence,
		seedGrowthAmount: cfg.SeedGrowthAmount,
		shutdownChan:     cfg.ShutdownChan,
		scheduleChan:     cfg.SchedulerChan,
		growthFactor:     0,
		tests:            tests,
		testConfig:       cfg.TestConfig,
		trackedFiles:     make(map[string]bool),
	}
}

func (ts *TestScheduler) Run() {
	keepRunning := true
	ts.seedResetTime = time.Now().Add(ts.seed.Duration)
	for keepRunning {
		ts.ScheduleTest()
		select {
		case _, keepRunning = <-ts.shutdownChan:
		default:
		}
		time.Sleep(time.Microsecond * 50)
	}
}

// ScheduleTest schedules a random test on the channel if we haven't met our quota based on seed configs
func (ts *TestScheduler) ScheduleTest() {
	targetSeed := ts.seed.TestsPerDuration + (ts.growthFactor * ts.seedGrowthAmount)

	// If we are after our reset time, reset to a new time, and reset num scheduled to whatever'se left, or 0
	if time.Now().UnixMicro() > ts.seedResetTime.UnixMicro() {
		ts.seedResetTime = time.Now().Add(ts.seed.Duration)
		ts.numScheduled = targetSeed - ts.numScheduled
		ts.growthFactor++
		log.Infof("Now scheduling: %d req/sec", ts.seed.TestsPerDuration+(ts.growthFactor*ts.seedGrowthAmount))
	}

	if ts.numScheduled < targetSeed {
		ts.scheduleChan <- ts.GetTestFunc()
		ts.numScheduled++
		ts.totalScheduled++
		return
	}

}

// GetTestFunc selects a psuedo random test function to run
func (ts *TestScheduler) GetTestFunc() Test {
	rand.Seed(time.Now().Unix())
	createNewFile := rand.Intn(ts.testConfig.MaxFileCount) > len(ts.trackedFiles)
	var fileName string
	var testToRun = Test{
		fileName: fileName,
	}

	if createNewFile {
		testToRun.fileName = RandStringBytes(15)
		ts.trackedFiles.Add(fileName)
		testToRun.TestType = PUT
	} else {
		testId := rand.Intn(len(ts.tests))
		testToRun.fileName = ts.trackedFiles.RandomFile()
		if ts.tests[testId] == DELETE {
			ts.trackedFiles.Delete(fileName)
		}
		testToRun.TestType = ts.tests[testId]
	}

	return testToRun
}
