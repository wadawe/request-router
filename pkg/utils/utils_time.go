// utils_time.go
// This file contains utility functions related to time and durations

package utils

import "time"

// Convert a string to a time.Duration
func ConvertToDuration(timeout string, defaultTimeout string) (time.Duration, error) {
	timeoutString := timeout
	if timeoutString == "" {
		timeoutString = defaultTimeout
	}
	duration, err := time.ParseDuration(timeoutString)
	if err != nil {
		return 0, err
	}
	return duration, nil
}
