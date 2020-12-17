package app

import (
	log "github.com/sirupsen/logrus"
)

var Log = setupLogger()

func setupLogger() *log.Logger {
	logger := log.New()
	logger.SetReportCaller(true)
	logger.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	return logger
}
