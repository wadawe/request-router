// config.go
// This file contains the top-level configuration functions for the request router

package config

import (
	"fmt"
	"log"
	"os"

	"github.com/BurntSushi/toml"
)

type GlobalSettings struct {
	LogDir    string
	UserAgent string
	Version   string
}

var Global = &GlobalSettings{}

// Set the log directory
func SetLogDir(dir string) {
	Global.LogDir = dir
}

// Get the log directory
func GetLogDir() string {
	return Global.LogDir
}

// Set the user agent
func SetUserAgent(agent string) {
	Global.UserAgent = agent
}

// Get the user agent
func GetUserAgent() string {
	return Global.UserAgent
}

// Set the version
func SetVersion(version string) {
	Global.Version = version
}

// Get the version
func GetVersion() string {
	return Global.Version
}

// Read and validate a configuration file
func ReadConfig(filename string) (*ConfigFile, error) {
	log.Printf("Reading config from %s", filename)
	var config = &ConfigFile{}
	if filename == "" {
		return nil, fmt.Errorf("no configuration file specified")
	}

	openFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer openFile.Close()

	// Decode the TOML config into our struct
	if _, err := toml.NewDecoder(openFile).Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode TOML: %w", err)
	}
	config.ApplyDefaults()
	if err = config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

// Get a connection config by name
func (cfg *ConfigFile) GetConnectionConfig(name string) *ConnectionConfig {
	for _, iCfg := range cfg.ConnectionConfigs {
		if iCfg.Name == name {
			return iCfg
		}
	}
	return nil
}

// Get a service config by name
func (cfg *ConfigFile) GetServiceConfig(name string) *ServiceConfig {
	for _, sCfg := range cfg.ServiceConfigs {
		if sCfg.Name == name {
			return sCfg
		}
	}
	return nil
}
