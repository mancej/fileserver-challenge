package main

import (
	"FileServerChallenge/internal"
	log "github.com/sirupsen/logrus"
	"time"
)

func main() {

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	start := time.Now()

	log.Infof("Starting FileServer.")
	fs := internal.NewFileServer()
	log.Fatal(fs.Run())

	finish := time.Now()
	totalTime := start.Sub(finish)
	log.Infof("Finished in %f seconds.", totalTime.Seconds())
}
