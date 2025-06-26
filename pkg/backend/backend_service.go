// backend_service.go
// This file contains the functions for creating and managing a backend service

package backend

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/wadawe/request-router/pkg/config"
)

type BackendService struct {
	Name    string   // The name of the service
	Members []string // The connection names for the service
}

// Create a new service handler
func NewBackendService(cfg *config.ServiceConfig) *BackendService {
	log.Printf("Creating new backend service: %s", cfg.Name)
	return &BackendService{
		Name:    cfg.Name,
		Members: cfg.Members,
	}
}

// Get all service connections
func (bs *BackendService) GetConnections() map[string]*BackendConnection {
	conns := make(map[string]*BackendConnection)
	for _, connName := range bs.Members {
		conn := GetBackendConnection(connName)
		if conn != nil {
			conns[connName] = conn
		}
	}
	return conns
}

// Get a connection from the service handler based on ping responses
func (bs *BackendService) GetFastestHealthyConnection() (*BackendConnection, error) {
	conns := bs.GetConnections()
	if len(conns) == 0 {
		return nil, fmt.Errorf("no available connections found")
	}

	var wg sync.WaitGroup
	wg.Add(len(conns))
	var responses = make(chan string, len(conns))

	// Send a ping request to each connection
	for _, conn := range conns {
		go func(bc *BackendConnection) {
			defer wg.Done()
			resp, err := bc.SendRequest(http.MethodGet, bc.PingEndpoint, nil, nil, nil, nil)
			if err != nil {
				return
			}

			// If the response is OK, update the healthy channel with the connection name
			if resp.StatusCode/100 == 2 {
				responses <- bc.Name
			}
		}(conn)
	}

	// Wait for all ping requests to complete
	// Close the responses channel when all requests are complete
	go func() {
		wg.Wait()
		close(responses)
	}()

	// Wait for the first response from the healthy channel
	fastestResponse, ok := <-responses
	if !ok {
		return nil, fmt.Errorf("no healthy connections found")
	}

	// Match the first healthy ping response to a connection
	fastestConn, exists := conns[fastestResponse]
	if !exists {
		return nil, fmt.Errorf("fastest connection (%s) does not exist in backend", fastestResponse)
	}
	return fastestConn, nil
}
