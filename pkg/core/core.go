// core.go
// This file contains the functions for creating and managing routers

package core

import (
	"fmt"
	"log"
	"sync"

	"github.com/wadawe/request-router/pkg/backend"
	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/core/router"
)

type RouterManager struct {
	Routers map[string]*router.ServiceRouter
}

// Create a new RouterManager
func NewRouterManager(cfg *config.ConfigFile) (*RouterManager, error) {

	// Create the RouterManager
	rMgr := &RouterManager{
		Routers: make(map[string]*router.ServiceRouter),
	}

	// Set the configuration for the backend
	err := backend.LoadConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Create ServiceRouters from the configuration file
	for _, rCfg := range cfg.RouterConfigs {
		if _, ok := rMgr.Routers[rCfg.BindAddress]; ok {
			return nil, fmt.Errorf("error on router (%s): duplicate bind address", rCfg.BindAddress)
		}
		sr, err := router.NewServiceRouter(rCfg)
		if err != nil {
			return nil, err
		}
		rMgr.Routers[sr.BindAddress] = sr
	}

	// Return the RouterManager
	return rMgr, nil
}

// Start the RouterManager
func (rMgr *RouterManager) Start() {
	var wg sync.WaitGroup
	wg.Add(len(rMgr.Routers))

	// Iterate Routers and start them
	for _, sr := range rMgr.Routers {

		// Async function to start the router service
		// Defer the wait group so that it is decremented when the function finishes
		go func(router *router.ServiceRouter) {
			defer wg.Done()
			log.Printf("Starting router on: %s", router.BindAddress)
			err := router.ListenAndServe()
			if err != nil {
				log.Printf("Error running router (%s): %v", router.BindAddress, err)
			}
		}(sr)
	}

	// Wait for all routers to finish
	wg.Wait()
	log.Printf("All routers have exited...")
}

// Stop the RouterManager
func (rMgr *RouterManager) Stop() {
	for _, sr := range rMgr.Routers {
		sr.Stop()
	}
}
