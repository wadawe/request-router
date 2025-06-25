// utils_logger.go
// This file contains utility functions related to logging

package utils

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/wadawe/request-router/pkg/config"
)

type LogHandler struct {
	Name string
	File *os.File
}

var (
	logHandlers []*LogHandler
)

// Setup the log directory if defined
func SetupLogDirectory(logDirFlag *string) {
	var err error
	var logDir string
	if len(*logDirFlag) > 0 { // Log directory specified in command line
		logDir = *logDirFlag
	} else { // Use current working directory
		logDir, err = os.Getwd()
		if err != nil {
			log.Fatal(err)
		}
	}
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err = os.Mkdir(logDir, 0755)
		if err != nil {
			log.Fatalf("Error on log directory creation (%s): %s", logDir, err)
		}
	}
	log.Printf("Using log directory: %s", logDir)
	config.SetLogDir(logDir)
}

// Create a new logger
func NewFileLogger(logFile string, logLevel string) *zerolog.Logger {
	var loggerOutput *os.File

	// Check if log file is provided
	if len(logFile) > 0 {
		filename := logFile
		if !filepath.IsAbs(logFile) {
			filename = filepath.Join(config.GetLogDir(), filename)
		}
		file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Fatalf("Error on log file open (%s): %s", filename, err)
		}

		// Store the logger
		// We need to close this file when the program exits
		loggerOutput = file
		logHandlers = append(logHandlers, &LogHandler{Name: filename, File: loggerOutput})

	} else {
		// If no log file is provided, set the output to stderr
		// We don't need to store this in the logHandlers list as we don't need to close it
		loggerOutput = os.Stderr
	}

	// Create the log writer
	writer := zerolog.ConsoleWriter{Out: loggerOutput, TimeFormat: time.RFC3339, NoColor: true}
	zerolog.TimeFieldFormat = time.RFC3339
	fw := zerolog.New(writer).With().Timestamp().Logger()
	if logLevel == "" {
		logLevel = "info" // Default log level if not specified
	}
	return setLoggerLevel(&fw, logLevel)
}

// Set the logger level for a logger
func setLoggerLevel(logger *zerolog.Logger, level string) *zerolog.Logger {
	var l zerolog.Logger
	switch strings.ToLower(level) {
	case "panic":
		l = logger.Level(zerolog.PanicLevel)
	case "fatal":
		l = logger.Level(zerolog.FatalLevel)
	case "error":
		l = logger.Level(zerolog.ErrorLevel)
	case "warn", "warning":
		l = logger.Level(zerolog.WarnLevel)
	case "info", "informational":
		l = logger.Level(zerolog.InfoLevel)
	case "debug":
		l = logger.Level(zerolog.DebugLevel)
	default:
		l = logger.Level(zerolog.InfoLevel)
	}
	return &l
}

// Close all log handlers
func CloseLogHandlers() {
	for _, handler := range logHandlers {
		handler.File.Close()
	}
}
