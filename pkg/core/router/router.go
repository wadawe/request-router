// router.go
// This file contains the functions for creating a new router and starting it

package router

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/core/context"
	"github.com/wadawe/request-router/pkg/utils"
)

type HttpServerFunction func(*ServiceRouter) *http.Server

type PathHandler struct {
	Paths   map[string]*RouterPath // Map of RouterPath by path endpoint
	Methods string                 // Pre-calculated string of HTTP methods available for the path
}

type ServiceRouter struct {
	Config       *config.RouterConfig    // Configuration for the router
	TlsCerts     *tls.Certificate        // TLS certificates for the router
	Handlers     map[string]*PathHandler // Map of PathHandler by path endpoint
	AccessLogger *zerolog.Logger         // Access logger for the router
	Server       *http.Server            // HTTP server for the router
}

// Create a new ServiceRouter
func NewServiceRouter(cfg *config.RouterConfig) (*ServiceRouter, error) {
	sr := &ServiceRouter{
		Config:       cfg,
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

		// Create a new RouterPath for each method in the path configuration
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

	// Collapse to comma-separated string for each endpoint, ready to be used for the OPTIONS response
	// We precompute this to avoid recalculating it for every request
	for endpoint, methods := range methodsPerEndpoint {
		sr.Handlers[endpoint].Methods = strings.Join(methods, ", ")
	}

	// Set the server certificate and key if provided
	if cfg.ServerCert != "" && cfg.ServerKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ServerCert, cfg.ServerKey)
		if err != nil {
			return nil, err
		}
		sr.TlsCerts = &cert
	}

	// Set the HTTP server for the ServiceRouter
	var httpServer = map[config.HttpVersion]HttpServerFunction{
		config.HttpVersion_1_1: NewHttpServer_1_1,
		config.HttpVersion_2:   NewHttpServer_2,
		// Add more versions as needed
	}
	if fn, ok := httpServer[cfg.HttpVersion]; ok {
		sr.Server = fn(sr)
	} else {
		return nil, fmt.Errorf("error on router (%s): unhandled HTTP version (%s)", sr.Config.BindAddress, cfg.HttpVersion)
	}

	// Return the new ServiceRouter
	return sr, nil
}

// Start the ServiceRouter
func (sr *ServiceRouter) ListenAndServe() error {
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
	context.AppendToContextTrace(r, "router", sr.Config.BindAddress)

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

// Create a new HTTP server for HTTP version 1.1
func NewHttpServer_1_1(sr *ServiceRouter) *http.Server {
	if sr.TlsCerts != nil {
		return &http.Server{
			Addr:    sr.Config.BindAddress,
			Handler: sr,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{*sr.TlsCerts},
			},
		}
	}

	// Unencrypted HTTP/1.1 server
	return &http.Server{
		Addr:    sr.Config.BindAddress,
		Handler: sr,
	}
}

// Create a new HTTP server for HTTP version 2
func NewHttpServer_2(sr *ServiceRouter) *http.Server {
	if sr.TlsCerts != nil {
		return &http.Server{
			Addr:    sr.Config.BindAddress,
			Handler: sr,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{*sr.TlsCerts},
			},
		}
	}

	// Unencrypted HTTP/2 server
	log.Printf("WARNING: HTTP/2 router (%s) without TLS certificates, using unencrypted HTTP/2", sr.Config.BindAddress)
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	return &http.Server{
		Addr:      sr.Config.BindAddress,
		Handler:   sr,
		Protocols: &protocols,
	}
}
