// context.go
// This file contains the context functions for the router service
// Context is used to store and pass data between the different parts of the router service

package context

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/wadawe/request-router/pkg/admin"
	"github.com/wadawe/request-router/pkg/backend"
	"github.com/wadawe/request-router/pkg/utils"
)

type RequestContext struct {
	StartTime      time.Time    // Start time of the request
	EndTime        time.Time    // End time of the request
	RequestLength  int          // Length of the request body
	ResponseLength int          // Length of the response body
	RequestSource  string       // Source of the request
	RequestTarget  string       // Target of the request
	RequestPath    string       // Path of the request
	Trace          bytes.Buffer // Trace of the request
	StatusCode     int          // Status of the response sent back to the client
}

type ResponseData struct {
	Service    string
	StatusCode int
	Responses  []*backend.BackendResponse
}

type ContextKey string

// Key to store the context in each request
const RequestContextKey ContextKey = "RequestContext"

// Create a new request context and attach it to the request
func AddRequestContext(request *http.Request) *http.Request {
	now := time.Now()
	requestContext := &RequestContext{
		StartTime:      now,
		EndTime:        now,
		RequestLength:  0,
		ResponseLength: 0,
		RequestSource:  utils.GetSourceFromRequest(request),
		RequestTarget:  "",
		RequestPath:    "unmatched",
		Trace:          bytes.Buffer{},
		StatusCode:     0,
	}
	ctx := context.WithValue(request.Context(), RequestContextKey, requestContext)
	return request.WithContext(ctx)
}

// Get the request context from the request
func GetRequestContext(request *http.Request) *RequestContext {
	requestContext, ok := request.Context().Value(RequestContextKey).(*RequestContext)
	if ok {
		return requestContext
	}
	return nil
}

// Add a trace to a request context buffer
func AppendToContextTrace(request *http.Request, component string, value string) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {
		if requestContext.Trace.Len() > 0 {
			requestContext.Trace.WriteString(" -> ")
		}
		requestContext.Trace.WriteString(component + "(" + value + ")")
	}
}

// Set the response length in the request context
func SetContextResponseLength(request *http.Request, length int) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {
		requestContext.ResponseLength = length
	}
}

// Set the request target in the request context
func SetContextRequestTarget(request *http.Request, target string) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {
		requestContext.RequestTarget = target
	}
}

// Set the request path in the request context
func SetContextRequestPath(request *http.Request, path string) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {
		requestContext.RequestPath = path
	}
}

// Set the request context as served
func SetContextStatusCode(request *http.Request, status int) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {
		requestContext.StatusCode = status
		requestContext.EndTime = time.Now()
	}
}

// Log the request context
func LogRequestContext(request *http.Request, logger *zerolog.Logger) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {
		logger.Info().
			Str("method", request.Method).
			Str("request-url", request.URL.String()).
			Int("request-length", requestContext.RequestLength).
			Int("response-length", requestContext.ResponseLength).
			Str("response-time", requestContext.EndTime.Sub(requestContext.StartTime).String()).
			Str("trace", requestContext.Trace.String()).
			Int("status-code", requestContext.StatusCode).
			Str("source", requestContext.RequestSource).
			Str("user-agent", request.UserAgent()).
			Msg("Request:")
	}
}

// Observe the request context for metrics collection
func ObserveRequestContext(request *http.Request, router string) {
	requestContext := GetRequestContext(request)
	if requestContext != nil {

		// Extract relevant data for metrics
		statusCode := fmt.Sprintf("%d", requestContext.StatusCode)

		// Update: relay_requests_total
		admin.GetMetricsHandler().RequestsTotal.WithLabelValues(
			router,                       // router
			requestContext.RequestPath,   // path
			request.Method,               // method
			requestContext.RequestTarget, // target
			statusCode,                   // response status
		).Inc()

		// Update: relay_request_duration_seconds
		admin.GetMetricsHandler().RequestDurationSeconds.WithLabelValues(
			router,                       // router
			requestContext.RequestPath,   // path
			request.Method,               // method
			requestContext.RequestTarget, // target
			statusCode,                   // response status
		).Observe(requestContext.EndTime.Sub(requestContext.StartTime).Seconds())

	}
}

// Return a status code to the client
func ReturnStatusCode(writer http.ResponseWriter, request *http.Request, status int) {
	writer.WriteHeader(status)
	SetContextStatusCode(request, status)
}

// Return a connection response to the client
func ReturnResponseData(writer http.ResponseWriter, request *http.Request, response *ResponseData) {

	// Find the first response with response status code
	var returnResponse *backend.BackendResponse
	for _, br := range response.Responses {
		if br.StatusCode == response.StatusCode {
			returnResponse = br
			break
		}
	}

	// Validate the response
	if returnResponse == nil {
		text := fmt.Sprintf("unable to find status code (%d) in service status codes: %s", response.StatusCode, response.JoinStatusCodes())
		ReturnResponseText(writer, request, http.StatusInternalServerError, text)
		return
	}

	// Set the response headers
	responseLength := len(returnResponse.Body)
	writer.Header().Set("Content-Length", fmt.Sprint(responseLength))
	writer.Header().Set("Content-Type", returnResponse.ContentType)
	writer.Header().Set("X-Response-Statuses", response.JoinStatusCodes())
	writer.Header().Set("X-Request-Id", returnResponse.Headers.Get("X-Request-Id"))

	// Write the response back to the client
	writer.WriteHeader(response.StatusCode)
	writer.Write(returnResponse.Body)
	SetContextResponseLength(request, responseLength)
	SetContextStatusCode(request, response.StatusCode)
}

// Return a text response to the client
func ReturnResponseText(writer http.ResponseWriter, request *http.Request, status int, message string) {
	responseLength := len(message)
	writer.Header().Set("Content-Length", fmt.Sprint(responseLength))
	writer.Header().Set("Content-Type", "text/plain")
	writer.Header().Set("X-Response-Text", message) // Add the message to headers in case of 204 status
	writer.WriteHeader(status)
	writer.Write([]byte(message))
	AppendToContextTrace(request, "response", message)
	SetContextResponseLength(request, responseLength)
	SetContextStatusCode(request, status)
}

// Return a JSON response to the client
func ReturnResponseJSON(writer http.ResponseWriter, request *http.Request, status int, response interface{}) {

	// Encode the data
	data, err := json.Marshal(response)
	if err != nil {
		text := fmt.Sprintf("unable to encode JSON response: %s", err)
		http.Error(writer, text, http.StatusInternalServerError)
		AppendToContextTrace(request, "error", text)
		SetContextStatusCode(request, http.StatusInternalServerError)
		return
	}

	// Write the response
	responseLength := len(data)
	writer.Header().Set("Content-Length", fmt.Sprint(responseLength))
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(data)
	SetContextResponseLength(request, responseLength)
	SetContextStatusCode(request, status)
}

// Join the status codes into a string of comma separated values
func (responseData *ResponseData) JoinStatusCodes() string {
	statusCodes := make([]string, 0)
	for _, response := range responseData.Responses {
		statusCodes = append(statusCodes, fmt.Sprintf("%d", response.StatusCode))
	}
	return strings.Join(statusCodes, ",")
}
