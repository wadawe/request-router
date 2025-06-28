// config_schema.go
// This file contains the schema / structures for the router configuration

package config

type ConfigFile struct {
	ConnectionConfigs []*ConnectionConfig `toml:"connection"`
	ServiceConfigs    []*ServiceConfig    `toml:"service"`
	RouterConfigs     []*RouterConfig     `toml:"router"`
}

type ConnectionConfig struct {
	Name         string `toml:"name"`          // Name of the connection
	Location     string `toml:"location"`      // URL of the connection
	PingEndpoint string `toml:"ping-endpoint"` // Endpoint for the connection health checks
	Timeout      string `toml:"timeout"`       // Request timeout for the connection
	ClientCert   string `toml:"client-cert"`   // Client certificate for the connection
	ClientKey    string `toml:"client-key"`    // Client key for the connection
}

type ServiceConfig struct {
	Name    string   `toml:"name"`    // Name of the service
	Members []string `toml:"members"` // List of connections in the service
}

type RouterConfig struct {
	BindAddress   string        `toml:"bind"`         // Address to bind the router to
	AccessLogFile string        `toml:"access-log"`   // Access log file for the router
	Paths         []*PathConfig `toml:"path"`         // List of paths to route requests through
	HttpVersion   HttpVersion   `toml:"http-version"` // HTTP version to use for receiving requests
	ServerCert    string        `toml:"tls-cert"`     // TLS certificate file for the router
	ServerKey     string        `toml:"tls-key"`      // TLS key file for the router
}

type PathConfig struct {
	Name             string          `toml:"name"`              // Name of the path
	Methods          []string        `toml:"methods"`           // List of HTTP methods to accept
	IncomingEndpoint string          `toml:"incoming-endpoint"` // Incoming endpoint of the path
	Targets          []*TargetConfig `toml:"target"`            // List of targets for the path
}

type TargetConfig struct {
	Name             string          `toml:"name"`              // Name of the target
	TargetService    string          `toml:"service"`           // Destination service name
	TargetReplica    string          `toml:"replica"`           // Destination replica name
	UpstreamEndpoint string          `toml:"upstream-endpoint"` // Upstream endpoint override of the path
	RequestAction    RequestAction   `toml:"request-action"`    // Request action of the target
	RequestStrategy  RequestStrategy `toml:"request-strategy"`  // Request strategy of the target
	FilterStrategy   FilterStrategy  `toml:"filter-strategy"`   // Filter strategy of the target
	Filters          []*FilterConfig `toml:"request-filter"`    // List of filters to apply to the target
	Headers          []*HeaderConfig `toml:"header-override"`   // List of header overrides to set for the target
	LogFile          string          `toml:"log-file"`          // Log file for the target
}

type FilterConfig struct {
	Source     FilterSource `toml:"source"` // Source to match the filter key against
	MatchKey   string       `toml:"key"`    // Filter key to match against
	MatchRegex string       `toml:"match"`  // Regex value to match against
}

type HeaderConfig struct {
	Key   string `toml:"key"`   // Header key to set
	Value string `toml:"value"` // Header value to set
}

type HttpVersion string // The HTTP version to use for the router

const (
	HttpVersion_1_1 HttpVersion = "1.1" // HTTP/1.1
	HttpVersion_2   HttpVersion = "2"   // HTTP/2.0
)

type RequestAction string // The current status of the target

const (
	RequestAction_Forward  RequestAction = "forward"  // Target is enabled and will receive requests
	RequestAction_Reject   RequestAction = "reject"   // Target is disabled and will not receive requests
	RequestAction_Simulate RequestAction = "simulate" // Target is simulating being enabled
	RequestAction_Offload  RequestAction = "offload"  // Target is offloading requests to another service
)

type FilterStrategy string // What strategy does the target use match requests against filters

const (
	FilterStrategy_All FilterStrategy = "all" // All filters must match
	FilterStrategy_Any FilterStrategy = "any" // Any filter can match
)

type FilterSource string // The source of the filter key to match against

const (
	FilterSource_Headers FilterSource = "headers" // Filter is applied to the request headers
	FilterSource_Query   FilterSource = "query"   // Filter is applied to the request query parameters
)

type RequestStrategy string // What strategy does the target take when making requests

const (
	RequestStrategy_Ping     RequestStrategy = "ping"     // a ping request is made to all service members and the fastest connection to respond successfully is used for the request
	RequestStrategy_Primary  RequestStrategy = "primary"  // requests are made in parallel to all service members and only the response from the primary member (first in the members list) is returned to the client
	RequestStrategy_Sequence RequestStrategy = "sequence" // requests are made in sequence to all service members until a successful response is received, which is then returned to the client, else the highest response code is returned
	RequestStrategy_Success  RequestStrategy = "success"  // requests are made in parallel to all service members and only the first successful response is returned to the client
	RequestStrategy_Highest  RequestStrategy = "highest"  // requests are made in parallel to all service members and the response with the highest response code is returned to the client (e.g. 400 > 204)
)
