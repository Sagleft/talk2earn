package main

import (
	"log"
	"os"

	"github.com/google/logger"
)

var logsHandler *logger.Logger
var logsFile *os.File

func initLogger() {
	var err error
	logsFile, err := os.OpenFile(logsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		logger.Fatalf("Failed to open log file: %v", err)
	}

	logger.SetFlags(log.Lshortfile)
	logsHandler = logger.Init("Logger", true, true, logsFile)
}
