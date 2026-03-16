// router.go
// This file contains the functions for creating a new router and starting it

package router

import (
	"crypto/tls"
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

		// Create a PathHandler for a specific endpoint, if it doesn't exist
		if sr.Handlers[pCfg.IncomingEndpoint] == nil {
			sr.Handlers[pCfg.IncomingEndpoint] = &PathHandler{
				Paths: make(map[string]*RouterPath),
			}
		}

		rp, err := NewRouterPath(pCfg)
		if err != nil {
			return nil, err
		}

		// Store the RouterPath in the PathHandler for each path method
		for _, method := range pCfg.Methods {
			upperMethod := strings.ToUpper(method)
			if upperMethod == http.MethodOptions {
				continue // skip OPTIONS
			}
			sr.Handlers[pCfg.IncomingEndpoint].Paths[upperMethod] = rp
			methodsPerEndpoint[pCfg.IncomingEndpoint] = append(methodsPerEndpoint[pCfg.IncomingEndpoint], upperMethod)
		}
	}

	// Collapse to comma-separated string for each endpoint, ready to be used for the OPTIONS response
	// We precompute this to avoid recalculating it for every request
	for endpoint, methods := range methodsPerEndpoint {
		methods = append(methods, http.MethodOptions) // Always include OPTIONS
		sr.Handlers[endpoint].Methods = strings.Join(methods, ", ")
	}

	if cfg.ServerCert != "" && cfg.ServerKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ServerCert, cfg.ServerKey)
		if err != nil {
			return nil, err
		}
		sr.TlsCerts = &cert
	}

	sr.Server = NewHttpServer(sr)

	// Return the new ServiceRouter
	return sr, nil
}

// Start the ServiceRouter
func (sr *ServiceRouter) ListenAndServe() error {
	var err error
	if sr.TlsCerts != nil {
		err = sr.Server.ListenAndServeTLS("", "")
	} else {
		err = sr.Server.ListenAndServe()
	}

	// Check for errors
	// ErrServerClosed always returned when server is closed gracefully
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Create a new HTTP server for the ServiceRouter
func NewHttpServer(sr *ServiceRouter) *http.Server {

	// TLS defined
	if sr.TlsCerts != nil {
		return &http.Server{
			Addr:    sr.Config.BindAddress,
			Handler: sr,
			TLSConfig: &tls.Config{
				Certificates: []tls.Certificate{*sr.TlsCerts},
			},
		}
	}

	// Force unencrypted HTTP/2 (h2c)
	if sr.Config.ForceH2C {
		log.Printf("WARNING: HTTP/2 router (%s) without TLS certificates, using h2c", sr.Config.BindAddress)
		var protocols http.Protocols
		protocols.SetUnencryptedHTTP2(true)
		return &http.Server{
			Addr:      sr.Config.BindAddress,
			Handler:   sr,
			Protocols: &protocols,
		}
	}

	// Default plain HTTP/1.1
	return &http.Server{
		Addr:    sr.Config.BindAddress,
		Handler: sr,
	}

}

// ServeHTTP implements the http.Handler interface for ServiceRouter
func (sr *ServiceRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r = context.AddRequestContext(r)
	context.AppendToContextTrace(r, "router", sr.Config.BindAddress)

	// Always log / observe the request
	// Defer is used to ensure that the request context is fully populated before logging / observing
	defer context.LogRequestContext(r, sr.AccessLogger)
	defer context.ObserveRequestContext(r, sr.Config.BindAddress)

	// Read the request body
	body, err := utils.ReadRequestBody(r)
	if err != nil {
		context.ReturnResponseText(w, r, http.StatusInternalServerError, "Unable to read request body: "+err.Error())
		return
	}

	// Match the request path to a PathHandler
	ph, ok := sr.Handlers[r.URL.Path]
	if !ok {
		context.ReturnResponseText(w, r, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}
	context.SetContextRequestPath(r, r.URL.Path)

	// Match the request method to a RouterPath
	if r.Method == http.MethodOptions {
		sr.HandleOptions(w, r, ph.Methods)
	} else if rp, ok := ph.Paths[r.Method]; ok {
		rp.HandleRequest(w, r, body)
	} else {
		context.ReturnResponseText(w, r, http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed))
	}

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
