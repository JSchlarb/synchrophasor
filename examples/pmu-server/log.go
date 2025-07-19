package main

import (
	log "github.com/sirupsen/logrus"
)

func setupLogging(logLevel string) {
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap: log.FieldMap{
			log.FieldKeyTime:  "timestamp",
			log.FieldKeyLevel: "level",
			log.FieldKeyMsg:   "message",
		},
	})

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.WithError(err).Warn("Invalid log level, defaulting to INFO")
		level = log.InfoLevel
	}
	log.SetLevel(level)
}
