// config_validation.go
// This file contains the functions for validating the configuration of the request router

package config

import (
	"fmt"
	"net/url"
	"regexp"
	"time"
)

// Detect duplicate names in the configuration
func DetectDuplicate(name string, seen map[string]bool) bool {
	if seen[name] {
		return true
	}
	seen[name] = true
	return false
}

// Apply defaults to a config file
func (cfg *ConfigFile) ApplyDefaults() {
	for _, cCfg := range cfg.ConnectionConfigs {
		cCfg.ApplyDefaults()
	}
	for _, sCfg := range cfg.ServiceConfigs {
		sCfg.ApplyDefaults()
	}
	for _, rCfg := range cfg.RouterConfigs {
		rCfg.ApplyDefaults()
		for _, pCfg := range rCfg.Paths {
			pCfg.ApplyDefaults()
			for _, tCfg := range pCfg.Targets {
				tCfg.ApplyDefaults(pCfg)
				for _, fCfg := range tCfg.Filters {
					fCfg.ApplyDefaults()
				}
				for _, hCfg := range tCfg.Headers {
					hCfg.ApplyDefaults()
				}
			}
		}
	}
}

// Validate the configuration of a config file
func (cfg *ConfigFile) Validate() error {
	entityNames := make(map[string]bool)
	routerBinds := make(map[string]bool)

	// Validate all connection configs
	if len(cfg.ConnectionConfigs) < 1 {
		return fmt.Errorf("error on connections: config should have '>=1' connections")
	}
	for _, cCfg := range cfg.ConnectionConfigs {
		if DetectDuplicate(cCfg.Name, entityNames) {
			return fmt.Errorf("error on connection: duplicate name found (%s)", cCfg.Name)
		}
		err := cCfg.Validate()
		if err != nil {
			return err
		}
	}

	// Validate all service configs
	if len(cfg.ServiceConfigs) < 1 {
		return fmt.Errorf("error on services: config should have '>=1' services")
	}
	for _, sCfg := range cfg.ServiceConfigs {
		if DetectDuplicate(sCfg.Name, entityNames) {
			return fmt.Errorf("error on service: duplicate name found (%s)", sCfg.Name)
		}
		err := sCfg.Validate(cfg)
		if err != nil {
			return err
		}
	}

	// Validate all router configs
	if len(cfg.RouterConfigs) < 1 {
		return fmt.Errorf("error on routers: config should have '>=1' routers")
	}
	for _, rCfg := range cfg.RouterConfigs {
		if DetectDuplicate(rCfg.BindAddress, routerBinds) {
			return fmt.Errorf("error on router: duplicate bind address found (%s)", rCfg.BindAddress)
		}
		for _, pCfg := range rCfg.Paths {
			if DetectDuplicate(pCfg.Name, entityNames) {
				return fmt.Errorf("error on path: duplicate name found (%s)", pCfg.Name)
			}
			for _, tCfg := range pCfg.Targets {
				if DetectDuplicate(tCfg.Name, entityNames) {
					return fmt.Errorf("error on target: duplicate name found (%s)", tCfg.Name)
				}
			}
		}
		err := rCfg.Validate(cfg)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Apply defaults to a connection config
func (cfg *ConnectionConfig) ApplyDefaults() {
	if cfg.PingEndpoint == "" {
		cfg.PingEndpoint = "/ping"
	}
	if cfg.Timeout == "" {
		cfg.Timeout = "10s"
	}
}

// Validate the configuration of a connection
func (cfg *ConnectionConfig) Validate() error {
	var err error

	// Validate connection name
	if cfg.Name == "" {
		return fmt.Errorf("error on connection: name is empty")
	}

	// Validate connection location
	if cfg.Location == "" {
		return fmt.Errorf("error on connection (%s): location is empty", cfg.Name)
	}
	_, err = url.Parse(cfg.Location)
	if err != nil {
		return fmt.Errorf("error on connection (%s): location is invalid (%s): %s", cfg.Name, cfg.Location, err)
	}

	// Validate connection timeout
	_, err = time.ParseDuration(cfg.Timeout)
	if err != nil {
		return fmt.Errorf("error on connection (%s): timeout is invalid (%s): %s", cfg.Name, cfg.Timeout, err)
	}

	// Validate client certificate and key
	if cfg.ClientCert != "" && cfg.ClientKey == "" {
		return fmt.Errorf("error on connection (%s): client cert is set without client key", cfg.Name)
	}
	if cfg.ClientCert == "" && cfg.ClientKey != "" {
		return fmt.Errorf("error on connection (%s): client key is set without client cert", cfg.Name)
	}

	// Config is valid
	return nil
}

// Apply defaults to a service config
func (cfg *ServiceConfig) ApplyDefaults() {
	// ...
}

// Validate the configuration of a service
func (cfg *ServiceConfig) Validate(root *ConfigFile) error {

	// Validate service name
	if cfg.Name == "" {
		return fmt.Errorf("error on service: name is empty")
	}

	// Validate service members
	if len(cfg.Members) < 1 {
		return fmt.Errorf("error on service (%s): should have '>=1' members", cfg.Name)
	}
	for _, member := range cfg.Members {
		if root.GetConnectionConfig(member) == nil {
			return fmt.Errorf("error on service (%s) members: unknown connection (%s)", cfg.Name, member)
		}
	}

	// Config is valid
	return nil
}

// Apply defaults to a router config
func (cfg *RouterConfig) ApplyDefaults() {
	// ...
}

// Validate the configuration of a router
func (cfg *RouterConfig) Validate(root *ConfigFile) error {
	endpoints := make(map[string]map[string]bool)

	// Validate bind address
	if cfg.BindAddress == "" {
		return fmt.Errorf("error on router: bind address is empty")
	}

	// Validate server certificate and key
	if cfg.ServerCert != "" && cfg.ServerKey == "" {
		return fmt.Errorf("error on router (%s): tls cert is set without tls key", cfg.BindAddress)
	}
	if cfg.ServerCert == "" && cfg.ServerKey != "" {
		return fmt.Errorf("error on router (%s): tls key is set without tls cert", cfg.BindAddress)
	}

	// Validate router paths
	if len(cfg.Paths) < 1 {
		return fmt.Errorf("error on router (%s): config should have '>=1' paths", cfg.BindAddress)
	}
	for _, pCfg := range cfg.Paths {
		_, ok := endpoints[pCfg.IncomingEndpoint]
		if !ok {
			endpoints[pCfg.IncomingEndpoint] = map[string]bool{}
		}
		for _, method := range pCfg.Methods {
			if DetectDuplicate(method, endpoints[pCfg.IncomingEndpoint]) {
				return fmt.Errorf("error on path (%s): duplicate method (%s) found for endpoint (%s)", pCfg.Name, method, pCfg.IncomingEndpoint)
			}
		}

		// Validate the config
		err := pCfg.Validate(root)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Apply defaults to a path config
func (cfg *PathConfig) ApplyDefaults() {
	// ...
}

// Validate the configuration of a path
func (cfg *PathConfig) Validate(root *ConfigFile) error {

	// Validate path name
	if cfg.Name == "" {
		return fmt.Errorf("error on path: name is empty")
	}

	// Validate path methods
	if len(cfg.Methods) < 1 {
		return fmt.Errorf("error on path (%s): config should have '>=1' methods", cfg.IncomingEndpoint)
	}

	// Validate path endpoint
	if cfg.IncomingEndpoint == "" {
		return fmt.Errorf("error on path (%s): endpoint is empty", cfg.IncomingEndpoint)
	}

	// Validate path targets
	if len(cfg.Targets) < 1 {
		return fmt.Errorf("error on path (%s): should have '>=1' targets", cfg.IncomingEndpoint)
	}
	for _, tCfg := range cfg.Targets {
		err := tCfg.Validate(root, cfg.IncomingEndpoint)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Apply defaults to a target config
func (cfg *TargetConfig) ApplyDefaults(path *PathConfig) {
	if cfg.UpstreamEndpoint == "" {
		cfg.UpstreamEndpoint = path.IncomingEndpoint
	}
	if cfg.RequestAction == "" {
		cfg.RequestAction = RequestAction_Forward
	}
	if cfg.RequestStrategy == "" {
		cfg.RequestStrategy = RequestStrategy_Highest
	}
	if cfg.FilterStrategy == "" {
		cfg.FilterStrategy = FilterStrategy_Any // Doesn't matter, because we clear the filters below
		cfg.Filters = []*FilterConfig{}
	}
}

// Validate the configuration of a target
func (cfg *TargetConfig) Validate(root *ConfigFile, pathName string) error {

	// Validate target name
	if cfg.Name == "" {
		return fmt.Errorf("error on path (%s) target: name is empty", pathName)
	}

	// Validate target service
	sCfg := root.GetServiceConfig(cfg.TargetService)
	if sCfg == nil {
		return fmt.Errorf("error on target (%s): unknown service (%s)", cfg.Name, cfg.TargetService)
	}

	// Validate target replica
	if cfg.TargetReplica != "" {
		replicaCfg := root.GetServiceConfig(cfg.TargetReplica)
		if replicaCfg == nil {
			return fmt.Errorf("error on target (%s): unknown replica (%s)", cfg.Name, cfg.TargetReplica)
		}
	}

	// Validate target status
	if !isRequestAction(cfg.RequestAction) {
		return fmt.Errorf("error on target (%s): unknown request action (%s)", cfg.Name, cfg.RequestAction)
	}

	// Validate target request strategy
	if !isRequestStrategy(cfg.RequestStrategy) {
		return fmt.Errorf("error on target (%s): unknown request strategy (%s)", cfg.Name, cfg.RequestStrategy)
	}

	// Validate target filter strategy
	if !isFilterStrategy(cfg.FilterStrategy) {
		return fmt.Errorf("error on target (%s): unknown filter strategy (%s)", cfg.Name, cfg.FilterStrategy)
	}

	// Validate set headers
	for _, hCfg := range cfg.Headers {
		err := hCfg.Validate(cfg)
		if err != nil {
			return err
		}
	}

	// Validate target filters
	for _, fCfg := range cfg.Filters {
		err := fCfg.Validate(cfg)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Apply defaults to a filter config
func (cfg *FilterConfig) ApplyDefaults() {
	// ...
}

// Validate the configuration of a filter
func (cfg *FilterConfig) Validate(target *TargetConfig) error {

	// Validate filter context
	if !isFilterSource(cfg.Source) {
		return fmt.Errorf("error on target (%s) filter: unknown source (%s)", target.Name, cfg.Source)
	}

	// Validate filter key
	if cfg.MatchKey == "" {
		return fmt.Errorf("error on target (%s) filter: match key is empty", target.Name)
	}

	// Validate filter regex
	_, err := regexp.Compile(cfg.MatchRegex)
	if err != nil {
		return fmt.Errorf("error on target (%s) filter: match (%s) is invalid: %s", target.Name, cfg.MatchRegex, err)
	}

	// Config is valid
	return nil
}

// Apply defaults to a header config
func (cfg *HeaderConfig) ApplyDefaults() {
	// ...
}

// Validate the configuration of a header
func (cfg *HeaderConfig) Validate(target *TargetConfig) error {
	// Header values can be empty, so we don't validate them
	// Empty header values will delete the header from the request
	if cfg.Key == "" {
		return fmt.Errorf("error on target (%s) headers: header key is empty", target.Name)
	}
	return nil
}

// Verify if a RequestAction is valid
func isRequestAction(action RequestAction) bool {
	var validActions = map[RequestAction]struct{}{
		RequestAction_Forward:  {},
		RequestAction_Reject:   {},
		RequestAction_Simulate: {},
		RequestAction_Offload:  {},
		// Add more valid actions here as needed
	}
	_, ok := validActions[action]
	return ok
}

// Verify if a RequestStrategy is valid
func isRequestStrategy(strategy RequestStrategy) bool {
	var validStrategies = map[RequestStrategy]struct{}{
		RequestStrategy_Ping:     {},
		RequestStrategy_Primary:  {},
		RequestStrategy_Sequence: {},
		RequestStrategy_Success:  {},
		RequestStrategy_Highest:  {},
		// Add more valid strategies here as needed
	}
	_, ok := validStrategies[strategy]
	return ok
}

// Verify if a FilterStrategy is valid
func isFilterStrategy(strategy FilterStrategy) bool {
	var validStrategies = map[FilterStrategy]struct{}{
		FilterStrategy_All: {},
		FilterStrategy_Any: {},
		// Add more valid strategies here as needed
	}
	_, ok := validStrategies[strategy]
	return ok
}

// Verify if a FilterSource is valid
func isFilterSource(context FilterSource) bool {
	var validSources = map[FilterSource]struct{}{
		FilterSource_Headers: {},
		FilterSource_Query:   {},
		// Add more valid sources here as needed
	}
	_, ok := validSources[context]
	return ok
}
