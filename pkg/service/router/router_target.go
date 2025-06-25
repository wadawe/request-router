// router_target.go
// This file contains the functions for creating and managing path targets

package router

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/rs/zerolog"
	"github.com/wadawe/request-router/pkg/backend"
	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/service/context"
	"github.com/wadawe/request-router/pkg/utils"
)

type MatchFiltersFunction func(*http.Request) bool

type ActionRequestFunction func(http.ResponseWriter, *http.Request, []byte)

type UseStrategyFunction func(http.ResponseWriter, *http.Request, []byte, *backend.BackendService) (*context.ResponseData, error)

type PathTarget struct {
	Name          string                // Name of the target
	Endpoint      string                // The endpoint for the target
	Service       string                // Service name for the target
	Replica       string                // Replica name for the target
	Headers       map[string]string     // Headers to add to the request before sending it to the target
	Filters       []*TargetFilter       // List of filters to validate an incoming request with
	MatchFilters  MatchFiltersFunction  // Function to check if the request matches the filters
	ActionRequest ActionRequestFunction // Function to action a request for the target
	UseStrategy   UseStrategyFunction   // Function to use a strategy for the target
	TargetLogger  *zerolog.Logger       // Logger for the target
}

// Create a new PathTarget
func NewPathTarget(cfg *config.TargetConfig, pathName string, pathEndpoint string) (*PathTarget, error) {
	log.Printf("Creating new path target: %s", cfg.Name)
	pt := &PathTarget{
		Name:     cfg.Name,
		Endpoint: pathEndpoint,
		Service:  cfg.TargetService,
		Replica:  cfg.TargetReplica,
		Headers:  make(map[string]string),
		Filters:  make([]*TargetFilter, 0),
	}

	// Override the endpoint if specified in the configuration
	if cfg.UpstreamPath == "" {
		pt.Endpoint = cfg.UpstreamPath
	}

	// Set the header overrides for the target
	for _, header := range cfg.Headers {
		if len(header.Value) > 0 {
			pt.Headers[header.Key] = header.Value
		} else {
			pt.Headers[header.Key] = "" // Ensure empty headers are set
		}
	}

	// Set the target logger
	var filename string
	if cfg.LogFile == "" {
		filename = cfg.LogFile
	} else {
		filename = cfg.Name + ".log"
	}
	pt.TargetLogger = utils.NewFileLogger(filename, cfg.LogLevel)

	// Create TargetFilter handlers for each filter in the configuration
	for _, fCfg := range cfg.Filters {
		filter, err := NewTargetFilter(fCfg, pt.TargetLogger)
		if err != nil {
			return nil, fmt.Errorf("error on target (%s): %w", pt.Name, err)
		}
		pt.Filters = append(pt.Filters, filter)
	}

	// Set the MatchFilters function for the target
	var filterStrategyMap = map[config.FilterStrategy]MatchFiltersFunction{
		config.FilterStrategy_All: pt.MatchFilters_All,
		config.FilterStrategy_Any: pt.MatchFilters_Any,
		// Add more strategies as needed
	}
	if fn, ok := filterStrategyMap[cfg.FilterStrategy]; ok {
		pt.MatchFilters = fn
	} else {
		return nil, fmt.Errorf("error on target (%s): unhandled filter strategy (%s)", pt.Name, cfg.FilterStrategy)
	}

	// Set the ActionRequest function for the target
	var actionRequestMap = map[config.RequestAction]ActionRequestFunction{
		config.RequestAction_Forward:  pt.ActionRequest_Forward,
		config.RequestAction_Reject:   pt.ActionRequest_Reject,
		config.RequestAction_Simulate: pt.ActionRequest_Simulate,
		config.RequestAction_Offload:  pt.ActionRequest_Offload,
		// Add more actions as needed
	}
	if fn, ok := actionRequestMap[cfg.RequestAction]; ok {
		pt.ActionRequest = fn
	} else {
		return nil, fmt.Errorf("error on target (%s): unhandled request action (%s)", pt.Name, cfg.RequestAction)
	}

	// Set the UseStrategy function for the target
	var requestStrategyMap = map[config.RequestStrategy]UseStrategyFunction{
		config.RequestStrategy_Ping:     pt.UseStrategy_Ping,
		config.RequestStrategy_Sequence: pt.UseStrategy_Sequence,
		config.RequestStrategy_Primary:  pt.UseStrategy_Primary,
		config.RequestStrategy_Success:  pt.UseStrategy_Success,
		config.RequestStrategy_Highest:  pt.UseStrategy_Highest,
		// Add more strategies as needed
	}
	if fn, ok := requestStrategyMap[cfg.RequestStrategy]; ok {
		pt.UseStrategy = fn
	} else {
		return nil, fmt.Errorf("error on target (%s): unhandled request strategy (%s)", pt.Name, cfg.RequestStrategy)
	}

	// Return the new PathTarget
	return pt, nil
}

// Check if a request matches 'all' filters for the PathTarget
func (pt *PathTarget) MatchFilters_All(r *http.Request) bool {
	for _, filter := range pt.Filters {
		if !filter.DoesMatch(r) {
			return false
		}
	}
	return true
}

// Check if a request matches 'any' filters for the PathTarget
func (pt *PathTarget) MatchFilters_Any(r *http.Request) bool {
	for _, filter := range pt.Filters {
		if filter.DoesMatch(r) {
			return true
		}
	}
	return false
}

// Action a 'forward' request to the PathTarget
func (pt *PathTarget) ActionRequest_Forward(w http.ResponseWriter, r *http.Request, body []byte) {
	defer pt.ForwardToReplica(w, r, body)
	service := backend.GetBackendService(pt.Service)
	if service == nil {
		context.ReturnResponseText(w, r, http.StatusInternalServerError, fmt.Sprintf("No handler available for target service: %s", pt.Service))
		return
	}
	resp, err := pt.UseStrategy(w, r, body, service)
	if err != nil {
		context.ReturnResponseText(w, r, http.StatusInternalServerError, fmt.Sprintf("Failed to route request to target service (%s): %s", pt.Service, err))
		return
	}
	context.ReturnResponseData(w, r, resp)
}

// Action a 'reject' request for the PathTarget
func (pt *PathTarget) ActionRequest_Reject(w http.ResponseWriter, r *http.Request, body []byte) {
	context.ReturnResponseText(w, r, http.StatusServiceUnavailable, http.StatusText(http.StatusServiceUnavailable))
}

// Action a 'simulate' request for the PathTarget
func (pt *PathTarget) ActionRequest_Simulate(w http.ResponseWriter, r *http.Request, body []byte) {
	context.ReturnStatusCode(w, r, http.StatusNoContent)
}

// Action a 'offload' request for the PathTarget
func (pt *PathTarget) ActionRequest_Offload(w http.ResponseWriter, r *http.Request, body []byte) {
	defer pt.ForwardToReplica(w, r, body)
	context.ReturnStatusCode(w, r, http.StatusNoContent)
}

// Use the 'ping' strategy for the PathTarget
func (pt *PathTarget) UseStrategy_Ping(w http.ResponseWriter, r *http.Request, body []byte, service *backend.BackendService) (*context.ResponseData, error) {

	// Get a connection from the backend
	conn, err := service.GetFastestHealthyConnection()
	if err != nil {
		pt.LogError("error on connection", r, err, service.Name, nil, 0)
		return nil, err
	}
	context.AppendToContextTrace(r, "connection", conn.Name)

	// Send the request to the connection
	response, err := conn.SendRequest(r.Method, r.URL.Path, r.Header, pt.Headers, r.URL.Query(), body)
	if err != nil {
		pt.LogError("error on request", r, err, service.Name, nil, 0)
		return nil, err
	}
	if response.StatusCode/100 != 2 {
		pt.LogError("error on response", r, nil, service.Name, response.Body, response.StatusCode)
	}

	// Return the response
	return &context.ResponseData{
		Service:    service.Name,
		StatusCode: response.StatusCode,
		Responses:  []*backend.BackendResponse{response},
	}, nil
}

// Use the 'sequence' strategy for the PathTarget
func (pt *PathTarget) UseStrategy_Sequence(w http.ResponseWriter, r *http.Request, body []byte, service *backend.BackendService) (*context.ResponseData, error) {
	connections := service.GetConnections()
	var allResponses = []*backend.BackendResponse{}

	// Iterate through the connections in the service in sequence
	for _, conn := range connections {
		response, err := conn.SendRequest(r.Method, r.URL.Path, r.Header, pt.Headers, r.URL.Query(), body)
		if err != nil {
			pt.LogError("error on request", r, err, service.Name, nil, 0)
			continue
		}
		if response.StatusCode/100 != 2 {
			pt.LogError("error on response", r, nil, service.Name, response.Body, response.StatusCode)
		}

		// Add the response to the responses list
		allResponses = append(allResponses, response)

		// If the response from the current connection is successful, we can return the responses
		if response.StatusCode/100 == 2 {
			context.AppendToContextTrace(r, "connection", conn.Name)
			return &context.ResponseData{
				Service:    service.Name,
				StatusCode: response.StatusCode,
				Responses:  allResponses,
			}, nil
		}
	}

	// We can still return the responses if the no response was successful
	// The client needs to be notified of the highest status code received
	if len(allResponses) > 0 {

		// Get the highest status code from the responses
		highestResponse := allResponses[0]
		for _, response := range allResponses {
			if response.StatusCode > highestResponse.StatusCode {
				highestResponse = response
			}
		}
		context.AppendToContextTrace(r, "connection", highestResponse.Name)
		return &context.ResponseData{
			Service:    service.Name,
			StatusCode: highestResponse.StatusCode,
			Responses:  allResponses,
		}, nil
	}

	// No responses were received, return an error
	return nil, fmt.Errorf("no responses received from target")
}

// Use the 'primary' strategy for the PathTarget
func (pt *PathTarget) UseStrategy_Primary(w http.ResponseWriter, r *http.Request, body []byte, service *backend.BackendService) (*context.ResponseData, error) {
	connections := service.GetConnections()
	var wg sync.WaitGroup
	wg.Add(len(connections))
	var serverResponses = make(chan *backend.BackendResponse, len(connections))

	// Send the request to each connection
	// Do this concurrently in a goroutine for each connection
	primary := connections[service.Members[0]] // Get the primary connection based on the first member in the service
	if primary == nil {
		return nil, fmt.Errorf("no primary connection found for service: %s", service.Name)
	}
	for _, conn := range connections {
		go func(conn *backend.BackendConnection) {
			defer wg.Done()
			response, err := conn.SendRequest(r.Method, r.URL.Path, r.Header, pt.Headers, r.URL.Query(), body)
			if err != nil {
				pt.LogError("error on request", r, err, service.Name, nil, 0)
				return
			}
			if response.StatusCode/100 != 2 {
				pt.LogError("error on response", r, nil, service.Name, response.Body, response.StatusCode)
			}

			// Only send the response from the primary connection to the serverResponses channel
			if conn.Name == primary.Name {
				serverResponses <- response
			}
		}(conn)
	}

	// Wait for all serverResponses to be received
	// Close the responses channel when all requests are complete
	// Do this in a goroutine so it's non-blocking, and we can handle the responses as soon as they arrive (if any arrive)
	go func() {
		wg.Wait()
		close(serverResponses)
	}()

	// Wait for the primary response from the serverResponses channel
	primaryResponse, ok := <-serverResponses
	if !ok {
		return nil, fmt.Errorf("no primary response received from target")
	}
	context.AppendToContextTrace(r, "connection", primaryResponse.Name)
	return &context.ResponseData{
		Service:    service.Name,
		StatusCode: primaryResponse.StatusCode,
		Responses:  []*backend.BackendResponse{primaryResponse},
	}, nil
}

// Use the 'success' strategy for the PathTarget
func (pt *PathTarget) UseStrategy_Success(w http.ResponseWriter, r *http.Request, body []byte, service *backend.BackendService) (*context.ResponseData, error) {
	connections := service.GetConnections()
	var wg sync.WaitGroup
	wg.Add(len(connections))
	var serviceResponses = make(chan *backend.BackendResponse, len(connections))

	// Send the request to each connection
	// Do this concurrently in a goroutine for each connection
	for _, conn := range connections {
		go func(conn *backend.BackendConnection) {
			defer wg.Done()
			response, err := conn.SendRequest(r.Method, r.URL.Path, r.Header, pt.Headers, r.URL.Query(), body)
			if err != nil {
				pt.LogError("error on request", r, err, service.Name, nil, 0)
				return
			}
			if response.StatusCode/100 != 2 {
				pt.LogError("error on response", r, nil, service.Name, response.Body, response.StatusCode)
			}

			// Only send successful responses to the serviceResponses channel
			if response.StatusCode/100 == 2 {
				serviceResponses <- response
			}
		}(conn)
	}

	// Wait for all serviceResponses to be received
	// Close the responses channel when all requests are complete
	// Do this in a goroutine so it's non-blocking, and we can handle the responses as soon as they arrive (if any arrive)
	go func() {
		wg.Wait()
		close(serviceResponses)
	}()

	// Wait for the first success response from the serviceResponses channel
	successResponse, ok := <-serviceResponses
	if !ok {
		return nil, fmt.Errorf("no successful responses received from target")
	}
	context.AppendToContextTrace(r, "connection", successResponse.Name)
	return &context.ResponseData{
		Service:    service.Name,
		StatusCode: successResponse.StatusCode,
		Responses:  []*backend.BackendResponse{successResponse},
	}, nil
}

// Use the 'highest' strategy for the PathTarget
func (pt *PathTarget) UseStrategy_Highest(w http.ResponseWriter, r *http.Request, body []byte, service *backend.BackendService) (*context.ResponseData, error) {
	connections := service.GetConnections()
	var wg sync.WaitGroup
	wg.Add(len(connections))
	var serviceResponses = make(chan *backend.BackendResponse, len(connections))

	// Send the request to each connection
	// Do this concurrently in a goroutine for each connection
	for _, conn := range connections {
		go func(conn *backend.BackendConnection) {
			defer wg.Done()
			response, err := conn.SendRequest(r.Method, r.URL.Path, r.Header, pt.Headers, r.URL.Query(), body)
			if err != nil {
				pt.LogError("error on request", r, err, service.Name, nil, 0)
				return
			}
			if response.StatusCode/100 != 2 {
				pt.LogError("error on response", r, nil, service.Name, response.Body, response.StatusCode)
			}
			serviceResponses <- response
		}(conn)
	}

	// Wait for all serviceResponses to be received
	// Close the responses channel when all requests are complete
	wg.Wait()
	close(serviceResponses)

	// Collect all responses from the serviceResponses channel
	var allResponses []*backend.BackendResponse
	for response := range serviceResponses {
		allResponses = append(allResponses, response)
	}
	if len(allResponses) == 0 {
		return nil, fmt.Errorf("no responses received from target")
	}

	// Return the highest status code received across all responses
	highestResponse := allResponses[0]
	for _, response := range allResponses {
		if response.StatusCode > highestResponse.StatusCode {
			highestResponse = response
		}
	}
	context.AppendToContextTrace(r, "connection", highestResponse.Name)
	return &context.ResponseData{
		Service:    service.Name,
		StatusCode: highestResponse.StatusCode,
		Responses:  allResponses,
	}, nil
}

// Forward a request to the replica of the PathTarget
func (pt *PathTarget) ForwardToReplica(w http.ResponseWriter, r *http.Request, body []byte) {
	if len(pt.Replica) == 0 {
		return
	}
	replica := backend.GetBackendService(pt.Replica)
	if replica == nil {
		pt.LogError("error on replica", r, fmt.Errorf("no service handler found"), pt.Replica, nil, 0)
	} else {
		for _, member := range replica.Members {
			conn := backend.GetBackendConnection(member)
			if conn == nil {
				pt.LogError("error on replica", r, fmt.Errorf("no connection found for member: %s", member), pt.Replica, nil, 0)
				continue
			}
			go func(c *backend.BackendConnection) {
				resp, err := c.SendRequest(r.Method, pt.Endpoint, r.Header, pt.Headers, r.URL.Query(), body)
				if err != nil {
					pt.LogError("error on replica", r, err, c.Name, nil, 0)
				} else if resp.StatusCode/100 != 2 {
					pt.LogError("error on replica", r, nil, c.Name, resp.Body, resp.StatusCode)
				}
			}(conn)
		}
	}
}

// Log an error for the PathTarget
func (pt *PathTarget) LogError(prefix string, request *http.Request, err error, service string, response []byte, status int) {
	err = fmt.Errorf("%s: %w", prefix, err)
	pt.TargetLogger.Error().Err(err).
		Str("service", service).
		Str("response-body", string(response)).
		Int("response-status", status).
		Msg(fmt.Sprintf("error on target (%s):", pt.Name))
}
