// utils_http.go
// This file contains utility functions related to HTTP requests and responses

package utils

import (
	"io"
	"net/http"
)

// Extract the source from the request
func GetSourceFromRequest(request *http.Request) string {
	source := request.RemoteAddr
	if forwardedAddress := request.Header.Get("X-Forwarded-For"); forwardedAddress != "" {
		if len(source) > 0 {
			source += ", " // Append a comma if there is already a source
		}
		source += forwardedAddress
	}
	return source
}

// Read and close the body of a HTTP request
func ReadRequestBody(request *http.Request) ([]byte, error) {
	if request.Body == nil {
		return nil, nil
	}
	defer request.Body.Close()
	return io.ReadAll(request.Body)
}

// Read and close the body of a HTTP response
func ReadResponseBody(response *http.Response) ([]byte, error) {
	if response.Body == nil {
		return nil, nil
	}
	defer response.Body.Close()
	return io.ReadAll(response.Body)
}
