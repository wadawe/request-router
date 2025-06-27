// utils_logger.go
// This file contains utility functions related to logging

package utils

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"github.com/wadawe/request-router/pkg/config"
)

var (
	logFiles = make(map[string]*os.File) // map of filename => log file
)

// Setup the log directory if defined
func SetupLogDirectory(logDirFlag *string) {
	var err error
	var logDir string
	if len(*logDirFlag) > 0 {
		logDir = *logDirFlag
	} else {
		logDir, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err = os.Mkdir(logDir, 0755)
		if err != nil {
			log.Fatalf("Error creating log directory (%s): %s", logDir, err)
		}
	}
	log.Printf("Using log directory: %s", logDir)
	config.SetLogDir(logDir)
}

// Create or reuse a logger
func NewFileLogger(logFile string) *zerolog.Logger {
	var loggerOutput *os.File

	if logFile != "" {

		// Handle relative/absolute paths
		filename := logFile
		if !filepath.IsAbs(logFile) {
			filename = filepath.Join(config.GetLogDir(), logFile)
		}

		// Check if file already opened
		if file, ok := logFiles[filename]; ok {
			loggerOutput = file
		} else {
			file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				log.Fatalf("Error opening log file (%s): %s", filename, err)
			}
			logFiles[filename] = file
			loggerOutput = file
		}

	} else {
		loggerOutput = os.Stderr // No need to close stderr
	}

	writer := zerolog.ConsoleWriter{
		Out:        loggerOutput,
		TimeFormat: time.RFC3339,
		NoColor:    true,
	}
	zerolog.TimeFieldFormat = time.RFC3339
	fw := zerolog.New(writer).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	return &fw
}

// Close all log files
func CloseLogFiles() {
	for _, file := range logFiles {
		if file != nil {
			file.Close()
		}
	}
}
