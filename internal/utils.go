package internal

import (
	"math/rand"
	"time"
)

func ExampleFunction() {
	print("Example function!")
}

// RandomDurationBetween returns a random duration between min/max duration.
// If min > max, then max is returned.
func RandomDurationBetween(min time.Duration, max time.Duration) time.Duration {
	if min > max {
		return min
	}

	rand.Seed(time.Now().UnixNano())
	time.Sleep(time.Nanosecond * 1)
	num := rand.Int63n(max.Nanoseconds() - min.Nanoseconds())

	return time.Duration(num + min.Nanoseconds())
}
