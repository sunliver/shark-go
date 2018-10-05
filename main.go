package main

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("recover from panic log, err: %v", err)
		}
	}()

	log.Debug("Debug log")
	log.Info("Info log")
	log.Warn("Warn log")
	log.Error("Error log")
	log.Panic("Panic log")
}
