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

	"github.com/wadawe/request-router/pkg/backend"
	"github.com/wadawe/request-router/pkg/utils"
)

type RequestContext struct {
	StartTime  time.Time         // Start time of the request
	EndTime    time.Time         // End time of the request
	Data       map[string]string // Relevant data to store in the context
	Trace      bytes.Buffer      // Trace of the request
	StatusCode int               // Status of the response sent back to the client
}

type ResponseData struct {
	Service    string
	StatusCode int
	Responses  []*backend.BackendResponse
}

type ContextKey string

// Key to store the context in each request
const RequestContextKey ContextKey = "RequestContext"
const SourceDataKey string = "Source"
const RequestLengthDataKey string = "RequestLength"
const ResponseLengthDataKey string = "ResponseLength"

// Create a new request context and attach it to the request
func AddRequestContext(request *http.Request) *http.Request {
	now := time.Now()
	requestContext := &RequestContext{
		StartTime: now,
		EndTime:   now,
		Data: map[string]string{
			RequestLengthDataKey:  "0",
			ResponseLengthDataKey: "0",
			SourceDataKey:         utils.GetSourceFromRequest(request),
		},
		Trace:      bytes.Buffer{},
		StatusCode: 0,
	}
	ctx := context.WithValue(request.Context(), RequestContextKey, requestContext)
	return request.WithContext(ctx)
}

// Add a trace to a request context buffer
func AppendToContextTrace(request *http.Request, component string, value string) {
	requestContext := request.Context().Value(RequestContextKey).(*RequestContext)
	if requestContext != nil {
		if requestContext.Trace.Len() > 0 {
			requestContext.Trace.WriteString(" -> ")
		}
		requestContext.Trace.WriteString(component + "(" + value + ")")
	}
}

// Set a data value in the request context
func SetContextData(request *http.Request, key string, value string) {
	requestContext := request.Context().Value(RequestContextKey).(*RequestContext)
	if requestContext != nil {
		requestContext.Data[key] = value
	}
}

// Get a data value from the request context
func GetContextData(request *http.Request, key string) string {
	requestContext := request.Context().Value(RequestContextKey).(*RequestContext)
	if requestContext != nil {
		return requestContext.Data[key]
	}
	return ""
}

// Set the request context as served
func SetContextStatusCode(request *http.Request, status int) {
	requestContext := request.Context().Value(RequestContextKey).(*RequestContext)
	if requestContext != nil {
		requestContext.StatusCode = status
		requestContext.EndTime = time.Now()
	}
}

// Get the request context status code
func GetContextStatusCode(request *http.Request) int {
	requestContext := request.Context().Value(RequestContextKey).(*RequestContext)
	if requestContext != nil {
		return requestContext.StatusCode
	}
	return 0
}

// Log the request context
func LogRequestContext(request *http.Request, logger *zerolog.Logger) {
	requestContext := request.Context().Value(RequestContextKey).(*RequestContext)
	if requestContext != nil {
		logger.Info().
			Str("method", request.Method).
			Str("request-url", request.URL.String()).
			Str("request-length", requestContext.Data[RequestLengthDataKey]).
			Str("response-length", requestContext.Data[ResponseLengthDataKey]).
			Str("response-time", requestContext.EndTime.Sub(requestContext.StartTime).String()).
			Str("trace", requestContext.Trace.String()).
			Int("status-code", requestContext.StatusCode).
			Str("source", requestContext.Data[SourceDataKey]).
			Str("user-agent", request.UserAgent()).
			Msg("Request:")
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
	responseLength := fmt.Sprint(len(returnResponse.Body))
	writer.Header().Set("Content-Length", responseLength)
	writer.Header().Set("Content-Type", returnResponse.ContentType)
	writer.Header().Set("X-Response-Statuses", response.JoinStatusCodes())
	writer.Header().Set("X-Request-Id", returnResponse.Headers.Get("X-Request-Id"))

	// Write the response back to the client
	writer.WriteHeader(response.StatusCode)
	writer.Write(returnResponse.Body)
	SetContextData(request, ResponseLengthDataKey, responseLength)
	SetContextStatusCode(request, response.StatusCode)
}

// Return a text response to the client
func ReturnResponseText(writer http.ResponseWriter, request *http.Request, status int, message string) {
	responseLength := fmt.Sprint(len(message))
	writer.Header().Set("Content-Length", responseLength)
	writer.Header().Set("Content-Type", "text/plain")
	writer.Header().Set("X-Response-Text", message) // Add the message to headers in case of 204 status
	writer.WriteHeader(status)
	writer.Write([]byte(message))
	AppendToContextTrace(request, "response", message)
	SetContextData(request, ResponseLengthDataKey, responseLength)
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
	responseLength := fmt.Sprint(len(data))
	writer.Header().Set("Content-Length", responseLength)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	writer.Write(data)
	SetContextData(request, ResponseLengthDataKey, responseLength)
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
