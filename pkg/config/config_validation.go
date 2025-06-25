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
func DetectDuplicate(name string, seen map[string]bool, label string) error {
	if seen[name] {
		return fmt.Errorf("duplicate %s found: %s", label, name)
	}
	seen[name] = true
	return nil
}

// Validate the configuration of the router
func (cfg *ConfigFile) ValidateConfig() error {

	// Validate all connection configs
	connectionNames := make(map[string]bool)
	for _, connCfg := range cfg.ConnectionConfigs {
		if err := DetectDuplicate(connCfg.Name, connectionNames, "connection"); err != nil {
			return err
		}
		err := connCfg.validateConnectionConfig()
		if err != nil {
			return err
		}
	}

	// Validate all service configs
	serviceNames := make(map[string]bool)
	if len(cfg.ServiceConfigs) < 1 {
		return fmt.Errorf("error on services: config should have '>=1' services")
	}
	for _, serviceCfg := range cfg.ServiceConfigs {
		if err := DetectDuplicate(serviceCfg.Name, serviceNames, "service"); err != nil {
			return err
		}
		err := serviceCfg.validateServiceConfig(cfg)
		if err != nil {
			return err
		}
	}

	// Validate all router configs
	routerNames := make(map[string]bool)
	pathNames := make(map[string]bool)
	targetNames := make(map[string]bool)
	if len(cfg.RouterConfigs) < 1 {
		return fmt.Errorf("error on routers: config should have '>=1' routers")
	}
	for _, routerCfg := range cfg.RouterConfigs {
		if err := DetectDuplicate(routerCfg.BindAddress, routerNames, "router"); err != nil {
			return err
		}
		for _, pathCfg := range routerCfg.Paths {
			if err := DetectDuplicate(pathCfg.Name, pathNames, "path"); err != nil {
				return err
			}
			for _, targetCfg := range pathCfg.Targets {
				if err := DetectDuplicate(targetCfg.Name, targetNames, "target"); err != nil {
					return err
				}
			}
		}
		err := routerCfg.validateRouterConfig(cfg)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Validate the configuration of an Connection
func (connCfg *ConnectionConfig) validateConnectionConfig() error {

	// Validate connection name
	if connCfg.Name == "" {
		return fmt.Errorf("error on connection: name is empty")
	}

	// Validate connection location
	if connCfg.Location == "" {
		return fmt.Errorf("error on connection (%s): location is empty", connCfg.Name)
	}
	_, err := url.Parse(connCfg.Location)
	if err != nil {
		return fmt.Errorf("error on connection (%s): location is invalid (%s): %s", connCfg.Name, connCfg.Location, err)
	}

	// Validate connection timeout
	if connCfg.Timeout != "" {
		_, err := time.ParseDuration(connCfg.Timeout)
		if err != nil {
			return fmt.Errorf("error on connection (%s): timeout is invalid (%s): %s", connCfg.Name, connCfg.Timeout, err)
		}
	}

	// Validate client certificate and key
	if connCfg.ClientCert != "" && connCfg.ClientKey == "" {
		return fmt.Errorf("error on connection (%s): client cert is set without client key", connCfg.Name)
	}
	if connCfg.ClientCert == "" && connCfg.ClientKey != "" {
		return fmt.Errorf("error on connection (%s): client key is set without client cert", connCfg.Name)
	}

	// Config is valid
	return nil
}

// Validate the configuration of a Service
func (serviceCfg *ServiceConfig) validateServiceConfig(cfg *ConfigFile) error {

	// Validate service name
	if serviceCfg.Name == "" {
		return fmt.Errorf("error on service: name is empty")
	}

	// Validate service members
	if len(serviceCfg.Members) < 1 {
		return fmt.Errorf("error on service (%s): should have '>=1' members", serviceCfg.Name)
	}
	for _, member := range serviceCfg.Members {
		if cfg.GetConnectionConfig(member) == nil {
			return fmt.Errorf("error on service (%s) members: unknown connection (%s)", serviceCfg.Name, member)
		}
	}

	// Config is valid
	return nil
}

// Validate the configuration of a Router
func (routerCfg *RouterConfig) validateRouterConfig(config *ConfigFile) error {

	// Validate bind address
	if routerCfg.BindAddress == "" {
		return fmt.Errorf("error on router (%s): bind address is empty", routerCfg.BindAddress)
	}

	// Validate router paths
	endpoints := make(map[string]map[string]bool)
	if len(routerCfg.Paths) < 1 {
		return fmt.Errorf("error on router: config should have '>=1' paths")
	}
	for _, pathCfg := range routerCfg.Paths {
		methods, ok := endpoints[pathCfg.IncomingPath]
		if !ok {
			methods = map[string]bool{}
			endpoints[pathCfg.IncomingPath] = methods
		}
		for _, method := range pathCfg.Methods {
			if endpoints[pathCfg.IncomingPath][method] {
				return fmt.Errorf("duplicate endpoint+method found: %s %s", method, pathCfg.IncomingPath)
			}
			endpoints[pathCfg.IncomingPath][method] = true
		}

		// Validate the config
		err := pathCfg.validatePathConfig(config)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Validate the configuration of a Path
func (pathCfg *PathConfig) validatePathConfig(config *ConfigFile) error {

	// Validate path name
	if pathCfg.Name == "" {
		return fmt.Errorf("error on path: name is empty")
	}

	// Validate path methods
	if len(pathCfg.Methods) < 1 {
		return fmt.Errorf("error on path (%s): config should have '>=1' methods", pathCfg.IncomingPath)
	}

	// Validate path endpoint
	if pathCfg.IncomingPath == "" {
		return fmt.Errorf("error on path (%s): endpoint is empty", pathCfg.IncomingPath)
	}

	// Validate path targets
	if len(pathCfg.Targets) < 1 {
		return fmt.Errorf("error on path (%s): should have '>=1' targets", pathCfg.IncomingPath)
	}
	for _, targetCfg := range pathCfg.Targets {
		err := targetCfg.validateTargetConfig(config, pathCfg.IncomingPath)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Validate the configuration of a Target
func (targetCfg *TargetConfig) validateTargetConfig(cfg *ConfigFile, pathName string) error {

	// Validate target name
	if targetCfg.Name == "" {
		return fmt.Errorf("error on path (%s) target: name is empty", pathName)
	}

	// Validate target service
	serviceCfg := cfg.GetServiceConfig(targetCfg.TargetService)
	if serviceCfg == nil {
		return fmt.Errorf("error on path (%s) target (%s): unknown service (%s)", pathName, targetCfg.Name, targetCfg.TargetService)
	}

	// Validate target replica
	if targetCfg.TargetReplica != "" {
		replicaCfg := cfg.GetServiceConfig(targetCfg.TargetReplica)
		if replicaCfg == nil {
			return fmt.Errorf("error on path (%s) target (%s): unknown replica (%s)", pathName, targetCfg.Name, targetCfg.TargetReplica)
		}
	}

	// Validate target status
	if targetCfg.RequestAction == "" {
		targetCfg.RequestAction = RequestAction_Forward // Default action is to forward requests
	}
	if !isRequestAction(targetCfg.RequestAction) {
		return fmt.Errorf("error on path (%s) target (%s): unknown request action (%s)", pathName, targetCfg.Name, targetCfg.RequestAction)
	}

	// Validate target request strategy
	if !isRequestStrategy(targetCfg.RequestStrategy) {
		return fmt.Errorf("error on path (%s) target (%s): unknown request strategy (%s)", pathName, targetCfg.Name, targetCfg.RequestStrategy)
	}

	// Validate target filter strategy
	if !isFilterStrategy(targetCfg.FilterStrategy) {
		return fmt.Errorf("error on path (%s) target (%s): unknown filter strategy (%s)", pathName, targetCfg.Name, targetCfg.FilterStrategy)
	}

	// Validate set headers
	for _, headerCfg := range targetCfg.Headers {
		if headerCfg.Key == "" {
			return fmt.Errorf("error on path (%s) target (%s) headers: header key is empty", pathName, targetCfg.Name)
		}
		// Header values can be empty, so we don't validate them
		// Empty header values will delete the header from the request
	}

	// Validate target filters
	for _, filterCfg := range targetCfg.Filters {
		err := filterCfg.validateFilterConfig(pathName, targetCfg.TargetService)
		if err != nil {
			return err
		}
	}

	// Config is valid
	return nil
}

// Validate the configuration of a RouterFilter
func (filterCfg *FilterConfig) validateFilterConfig(pathName string, targetName string) error {

	// Validate filter context
	if !isFilterSource(filterCfg.Source) {
		return fmt.Errorf("error on path (%s) target (%s) filter: unknown source (%s)", pathName, targetName, filterCfg.Source)
	}

	// Validate filter key
	if filterCfg.MatchKey == "" {
		return fmt.Errorf("error on path (%s) target (%s) filter: match key is empty", pathName, targetName)
	}

	// Validate filter regex
	_, err := regexp.Compile(filterCfg.MatchRegex)
	if err != nil {
		return fmt.Errorf("error on path (%s) target (%s) filter: match (%s) is invalid: %s", pathName, targetName, filterCfg.MatchRegex, err)
	}

	// Config is valid
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
	if _, exists := validActions[action]; exists {
		return true
	}
	return false
}

// Verify if a FilterStrategy is valid
func isFilterStrategy(strategy FilterStrategy) bool {
	var validStrategies = map[FilterStrategy]struct{}{
		FilterStrategy_All: {},
		FilterStrategy_Any: {},
		// Add more valid strategies here as needed
	}
	if _, exists := validStrategies[strategy]; exists {
		return true
	}
	return false
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
	if _, exists := validStrategies[strategy]; exists {
		return true
	}
	return false
}

// Verify if a FilterSource is valid
func isFilterSource(context FilterSource) bool {
	var validSources = map[FilterSource]struct{}{
		FilterSource_Headers: {},
		FilterSource_Query:   {},
		// Add more valid sources here as needed
	}
	if _, exists := validSources[context]; exists {
		return true
	}
	return false
}
