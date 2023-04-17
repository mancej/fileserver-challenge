package main

import (
	"github.com/mancej/fileserver-challenge/file_server/internal"
	log "github.com/sirupsen/logrus"
	"time"
)

func main() {

	log.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: false,
	})

	start := time.Now()

	log.Infof("Starting FileServer.")
	fs := internal.NewFileServer()
	log.Fatal(fs.Run())

	finish := time.Now()
	totalTime := finish.Sub(start)
	log.Infof("Finished in %f seconds.", totalTime.Seconds())
}
