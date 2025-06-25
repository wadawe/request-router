// router.go
// This file contains the functions for creating a new router and starting it

package router

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/service/context"
	"github.com/wadawe/request-router/pkg/utils"
)

type ServiceRouter struct {
	BindAddress  string                            // Bind address for the router
	Server       *http.Server                      // HTTP server for the router
	Paths        map[string]map[string]*RouterPath // Map of RouterPath by path endpoint and method
	AccessLogger *zerolog.Logger                   // Access logger for the router
}

// Create a new ServiceRouter
func NewServiceRouter(cfg *config.RouterConfig) (*ServiceRouter, error) {
	log.Printf("Creating new service router: %s", cfg.BindAddress)
	sr := &ServiceRouter{
		BindAddress: cfg.BindAddress,
		Paths:       make(map[string]map[string]*RouterPath),
	}
	sr.AccessLogger = utils.NewFileLogger(cfg.AccessLogFile, "debug")

	// Initialise Paths map
	for _, pCfg := range cfg.Paths {
		if sr.Paths[pCfg.IncomingPath] == nil {
			sr.Paths[pCfg.IncomingPath] = make(map[string]*RouterPath)
		}
		for _, method := range pCfg.Methods {
			method = strings.ToUpper(method) // Ensure method is uppercase
			if method == http.MethodOptions {
				continue // Skip OPTIONS method as it is handled separately
			}
			if _, exists := sr.Paths[pCfg.IncomingPath][method]; exists {
				return nil, fmt.Errorf("error on path (%s): duplicate method (%s) for endpoint: %s", pCfg.Name, method, pCfg.IncomingPath)
			}
			rp, err := NewRouterPath(pCfg, method, pCfg.IncomingPath)
			if err != nil {
				return nil, err
			}
			sr.Paths[pCfg.IncomingPath][method] = rp
		}
	}

	// Return the new router
	return sr, nil
}

// Start the ServiceRouter
func (sr *ServiceRouter) ListenAndServe() error {
	sr.Server = &http.Server{
		Addr:    sr.BindAddress,
		Handler: sr, // Handler requires .ServeHTTP() method
	}

	// Start the HTTP server
	err := sr.Server.ListenAndServe()

	// Check for errors
	// ErrServerClosed always returned when server is closed gracefully
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ServeHTTP implements the http.Handler interface for ServiceRouter
func (sr *ServiceRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = context.AddRequestContext(r)
	context.AppendToContextTrace(r, "router", sr.BindAddress)

	// Read the request body
	body, err := utils.ReadRequestBody(r)
	if err != nil {
		context.ReturnResponseText(w, r, http.StatusInternalServerError, "Unable to read request body: "+err.Error())
	} else {

		// Match the request
		if path, exists := sr.Paths[r.URL.Path]; exists {
			if r.Method == http.MethodOptions {
				sr.HandleOptions(w, r, utils.GetMapKeys(path))
			} else if rp, exists := path[r.Method]; exists {
				rp.HandleRequest(w, r, body)
			} else {
				context.ReturnResponseText(w, r, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
			}
		} else {
			context.ReturnResponseText(w, r, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		}

	}

	// Always log the request in the access logger
	context.LogRequestContext(r, sr.AccessLogger)
}

// Return the allowed methods for the request when handling OPTIONS requests
func (sr *ServiceRouter) HandleOptions(w http.ResponseWriter, r *http.Request, options []string) {
	w.Header().Set("Allow", strings.Join(options, ", "))
	w.WriteHeader(http.StatusNoContent)
	context.SetContextStatusCode(r, http.StatusNoContent)
}

// Stop the ServiceRouter gracefully
func (sr *ServiceRouter) Stop() error {
	if sr.Server != nil {
		return sr.Server.Close()
	}
	return nil
}
