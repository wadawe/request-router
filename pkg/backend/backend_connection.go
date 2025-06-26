// backend_connection.go
// This file contains the functions for making requests to a single backend connection

package backend

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/utils"
)

type BackendConnection struct {
	Name         string         // The name of the connection
	Location     string         // The base URL of connection
	PingEndpoint string         // The URL to ping for the connection
	Client       *http.Client   // The HTTP client to use for requests
	WG           sync.WaitGroup // Tracks in-flight requests for graceful shutdown
}

type BackendResponse struct {
	Name        string      // The name of the connection
	ContentType string      // The content type of the response
	StatusCode  int         // The response status code
	Headers     http.Header // The response headers
	Body        []byte      // The response body
}

var (
	defaultPingLocation = "/ping"
)

// Create a new reusable connection to a backend connection
func NewBackendConnection(name string, location string, ping string, timeout time.Duration, clientCert string, clientKey string) (*BackendConnection, error) {
	log.Printf("Creating new backend connection: %s", name)

	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than 0 for backend: %s", name)
	}

	keepAlive := 30 * time.Second
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     keepAlive,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: keepAlive,
		}).DialContext,
	}

	if clientCert != "" && clientKey != "" {
		cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	bConn := &BackendConnection{
		Name:         name,
		Location:     strings.TrimRight(location, "/"),  // Remove trailing slash
		PingEndpoint: "/" + strings.TrimLeft(ping, "/"), // Add leading slash
		Client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		WG: sync.WaitGroup{},
	}

	return bConn, nil
}

// Send a request to the backend connection
func (bConn *BackendConnection) SendRequest(method string, path string, reqHeaders http.Header, newHeaders map[string]string, query url.Values, body []byte) (*BackendResponse, error) {

	// Add to the client wait group
	// This is used to wait for all client requests to finish before .Release() is allowed to complete
	bConn.WG.Add(1)
	defer bConn.WG.Done()

	// Create a new body reader
	if body == nil {
		body = []byte{}
	}
	bodyReader := bytes.NewReader(body)

	// Create a new request
	req, err := http.NewRequest(method, bConn.Location+path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set the request headers
	if reqHeaders != nil {
		for hKey, hVal := range reqHeaders {
			for _, v := range hVal {
				req.Header.Add(hKey, v)
			}
		}
	}

	// Add any header overrides
	if newHeaders != nil {
		for key, value := range newHeaders {
			if value != "" {
				req.Header.Set(key, value)
			} else {
				req.Header.Del(key) // Remove the header if the value is empty
			}
		}
	}

	// Set mandatory headers
	req.Header.Set("User-Agent", config.GetUserAgent())
	req.Header.Set("X-Router-Version", config.GetVersion())

	// Add the query parameters
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}

	// Send the request to the backend connection
	resp, err := bConn.Client.Do(req)
	if err != nil {
		return nil, err
	}
	rBody, err := utils.ReadResponseBody(resp)
	if err != nil {
		return nil, err
	}
	return &BackendResponse{
		Name:        bConn.Name,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
		Headers:     resp.Header,
		Body:        rBody,
	}, nil
}

// Release the connection
func (bConn *BackendConnection) Release() {

	// Wait for all client requests to finish
	bConn.WG.Wait()

	// Close all HTTP client connections
	if bConn.Client != nil {
		bConn.Client.CloseIdleConnections()
	}

}
