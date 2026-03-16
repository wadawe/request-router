// backend_connection.go
// This file contains the functions for making requests to a single backend connection

package backend

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/utils"
)

type BackendConnection struct {
	Config *config.ConnectionConfig // Configuration for the connection
	Client *http.Client             // The HTTP client to use for requests
	WG     sync.WaitGroup           // Tracks in-flight requests for graceful shutdown
}

type BackendResponse struct {
	Name        string      // The name of the connection
	ContentType string      // The content type of the response
	StatusCode  int         // The response status code
	Headers     http.Header // The response headers
	Body        []byte      // The response body
}

// Create a new reusable connection to a backend connection
func NewBackendConnection(cfg *config.ConnectionConfig) (*BackendConnection, error) {

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

	if cfg.ClientCert != "" && cfg.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("timeout is invalid: %s", err)
	} else if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than 0")
	}

	return &BackendConnection{
		Config: cfg,
		Client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		WG: sync.WaitGroup{},
	}, nil
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
	req, err := http.NewRequest(method, bConn.Config.Location+path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set the request headers
	for hKey, hVal := range reqHeaders {
		for _, v := range hVal {
			req.Header.Add(hKey, v)
		}
	}

	// Add any header overrides
	for key, value := range newHeaders {
		if value != "" {
			req.Header.Set(key, value)
		} else {
			req.Header.Del(key) // Remove the header if the value is empty
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
		Name:        bConn.Config.Name,
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
