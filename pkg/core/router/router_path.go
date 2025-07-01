// router_path.go
// This file contains the functions for handling routing for specific paths

package router

import (
	"fmt"
	"net/http"

	"github.com/wadawe/request-router/pkg/config"
	"github.com/wadawe/request-router/pkg/core/context"
)

type RouterPath struct {
	Config  *config.PathConfig // Configuration for the path
	Targets []*PathTarget      // List of targets for the path
}

// Create a new RouterPath
// A single RouterPath corresponds to a specific path
func NewRouterPath(cfg *config.PathConfig) (*RouterPath, error) {

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
		Config:  cfg,
		Targets: targets,
	}, nil
}

// Handle a request to the RouterPath
func (rp *RouterPath) HandleRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	context.AppendToContextTrace(r, "path", rp.Config.Name)
	for _, target := range rp.Targets {
		if len(target.Filters) == 0 || target.MatchFilters(r) {
			context.AppendToContextTrace(r, "target", target.Config.Name)
			target.ActionRequest(w, r, body)
			return
		}
	}
	context.ReturnResponseText(w, r, http.StatusBadRequest, fmt.Sprintf("No targets matched for path: %s", rp.Config.Name))
}
