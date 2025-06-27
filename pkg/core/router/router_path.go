// router_path.go
// This file contains the functions for handling routing for specific paths

package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/core/context"
)

type RouterPath struct {
	Name     string        // Name of the path
	Method   string        // The HTTP method for the router path
	Endpoint string        // The incoming endpoint for the path router
	Targets  []*PathTarget // List of targets for the path
}

// Create a new RouterPath
func NewRouterPath(cfg *config.PathConfig, method string) (*RouterPath, error) {

	// Create PathTarget handlers for each target in the configuration
	targets := make([]*PathTarget, 0, len(cfg.Targets))
	for _, tCfg := range cfg.Targets {
		pt, err := NewPathTarget(tCfg)
		if err != nil {
			return nil, err
		}
		targets = append(targets, pt)
	}

	// Return the new RouterPath
	return &RouterPath{
		Name:     cfg.Name,
		Method:   method,
		Endpoint: "/" + strings.TrimLeft(cfg.IncomingEndpoint, "/"), // Ensure leading slash
		Targets:  targets,
	}, nil
}

// Handle a request to the RouterPath
func (rp *RouterPath) HandleRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	context.AppendToContextTrace(r, "path", rp.Name)
	for _, target := range rp.Targets {
		if len(target.Filters) == 0 || target.MatchFilters(r) {
			context.AppendToContextTrace(r, "target", target.Name)
			target.ActionRequest(w, r, body)
			return
		}
	}
	context.ReturnResponseText(w, r, http.StatusBadRequest, fmt.Sprintf("No targets matched for path: %s", rp.Name))
}
