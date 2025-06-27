// router.go
// This file contains the functions for creating a new router and starting it

package router

import (
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/core/context"
	"github.com/wadawe/request-router/pkg/utils"
)

type PathHandler struct {
	Paths   map[string]*RouterPath // Map of RouterPath by path endpoint
	Methods string                 // Pre-calculated string of HTTP methods available for the path
}

type ServiceRouter struct {
	BindAddress  string                  // Bind address for the router
	Server       *http.Server            // HTTP server for the router
	Handlers     map[string]*PathHandler // Map of PathHandler by path endpoint
	AccessLogger *zerolog.Logger         // Access logger for the router
}

// Create a new ServiceRouter
func NewServiceRouter(cfg *config.RouterConfig) (*ServiceRouter, error) {
	sr := &ServiceRouter{
		BindAddress:  cfg.BindAddress,
		Handlers:     make(map[string]*PathHandler),
		AccessLogger: utils.NewFileLogger(cfg.AccessLogFile),
	}

	// Temporary holder for building up method lists
	methodsPerEndpoint := make(map[string][]string)

	for _, pCfg := range cfg.Paths {
		if sr.Handlers[pCfg.IncomingEndpoint] == nil {
			sr.Handlers[pCfg.IncomingEndpoint] = &PathHandler{
				Paths:   make(map[string]*RouterPath),
				Methods: "",
			}
		}

		for _, method := range pCfg.Methods {
			upperMethod := strings.ToUpper(method)
			if upperMethod == http.MethodOptions {
				continue // skip OPTIONS
			}

			rp, err := NewRouterPath(pCfg, upperMethod)
			if err != nil {
				return nil, err
			}
			sr.Handlers[pCfg.IncomingEndpoint].Paths[upperMethod] = rp
			methodsPerEndpoint[pCfg.IncomingEndpoint] = append(methodsPerEndpoint[pCfg.IncomingEndpoint], upperMethod)
		}
	}

	// Collapse to comma-separated string for each endpoint
	for endpoint, methods := range methodsPerEndpoint {
		sr.Handlers[endpoint].Methods = strings.Join(methods, ", ")
	}

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
		if handler, ok := sr.Handlers[r.URL.Path]; ok {
			if r.Method == http.MethodOptions {
				sr.HandleOptions(w, r, handler.Methods)
			} else if rp, ok := handler.Paths[r.Method]; ok {
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
func (sr *ServiceRouter) HandleOptions(w http.ResponseWriter, r *http.Request, options string) {
	w.Header().Set("Allow", options)
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
