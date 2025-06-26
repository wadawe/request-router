// backend.go
// This file contains the functions for creating and managing connections to backend services

package backend

import (
	"fmt"
	"log"
	"sync/atomic"

	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/utils"
)

type BackendManager struct {
	Services    map[string]*BackendService
	Connections map[string]*BackendConnection
}

var (
	backendManager           atomic.Value // Holds the current active BackendManager for safe concurrent access
	defaultConnectionTimeout = "30s"
)

// Load the configuration for the backend service
func LoadConfig(cfg *config.ConfigFile) error {
	newMgr, err := NewBackendManager(cfg)
	if err != nil {
		return fmt.Errorf("error creating backend manager: %s", err)
	}

	// Put the new manager into the atomic value
	// This will replace the old manager if it exists
	oldMgr := backendManager.Swap(newMgr)
	log.Println("Backend configuration loaded!")
	if oldMgr != nil {
		oldMgr.(*BackendManager).Release()
	}
	return nil
}

// Create a new BackendManager
func NewBackendManager(cfg *config.ConfigFile) (*BackendManager, error) {
	var err error
	newMgr := new(BackendManager)

	newMgr.Connections = make(map[string]*BackendConnection)
	err = newMgr.loadBackendConnections(cfg)
	if err != nil {
		return nil, err
	}

	newMgr.Services = make(map[string]*BackendService)
	err = newMgr.loadBackendServices(cfg)
	if err != nil {
		return nil, err
	}

	return newMgr, nil
}

// Load services from a config for the backend manager
func (bm *BackendManager) loadBackendServices(cfg *config.ConfigFile) error {
	for _, sCfg := range cfg.ServiceConfigs {
		bm.Services[sCfg.Name] = NewBackendService(sCfg)
	}
	return nil
}

// Load connections from a config for the backend manager
func (bm *BackendManager) loadBackendConnections(cfg *config.ConfigFile) error {
	for _, cCfg := range cfg.ConnectionConfigs {
		timeout, err := utils.ConvertToDuration(cCfg.Timeout, defaultConnectionTimeout)
		if err != nil {
			return fmt.Errorf("error creating connection (%s) timeout: %s", cCfg.Name, err)
		}
		newConnection, err := NewBackendConnection(cCfg.Name, cCfg.Location, cCfg.PingEndpoint, timeout, cCfg.ClientCert, cCfg.ClientKey)
		if err != nil {
			return fmt.Errorf("error creating connection (%s): %s", cCfg.Name, err)
		}
		bm.Connections[cCfg.Name] = newConnection
	}
	return nil
}

// Release the backend service
func (bm *BackendManager) Release() {
	for _, connection := range bm.Connections {
		connection.Release()
	}
}

// Get the current backend service manager
func getManager() *BackendManager {
	return backendManager.Load().(*BackendManager)
}

// Get the backend handler for the specified service
func GetBackendService(name string) *BackendService {
	return getManager().Services[name]
}

// Get all backend handlers for all services
func GetBackendServices() map[string]*BackendService {
	return getManager().Services
}

// Get the backend handler for the specified connection
func GetBackendConnection(name string) *BackendConnection {
	return getManager().Connections[name]
}

// Get all backend handlers for all connections
func GetBackendConnections() map[string]*BackendConnection {
	return getManager().Connections
}
