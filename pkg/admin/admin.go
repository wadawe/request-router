package admin

import (
	"log"
	"net/http"

	"github.com/wadawe/request-router/pkg/config"
)

// Global variables
var (
	as *http.Server // Admin service
)

// Start the admin service
func Start(config *config.AdminConfig) {
	mux := http.NewServeMux()

	// Register handlers
	RegisterMetrics(mux)

	// Create the admin service
	as = &http.Server{
		Addr:    config.BindAddress,
		Handler: mux,
	}

	// Start the admin service
	log.Printf("Running admin: %s", config.BindAddress)
	go func() {
		err := as.ListenAndServe()

		// Check for errors
		// ErrServerClosed always returned when server is closed gracefully
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error running admin service (%s): %v", config.BindAddress, err)
		}
	}()
}

// Stop the admin service
func Stop() {
	if as != nil {
		err := as.Close()
		if err != nil {
			log.Printf("Error stopping admin service: %v", err)
		}
	}
}
