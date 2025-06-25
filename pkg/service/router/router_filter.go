// router_filter.go
// This file contains the functions for creating and matching target filters

package router

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/wadawe/request-router/pkg/config"
)

type DoesMatchFunction func(*http.Request) bool

type TargetFilter struct {
	DoesMatch    DoesMatchFunction // Function to check if the filter matches a request
	Key          string            // Key extracted from the request source
	Search       *regexp.Regexp    // Regex to match with the specified key with
	ParentLogger *zerolog.Logger   // Logger for the parent
}

func NewTargetFilter(cfg *config.FilterConfig, logger *zerolog.Logger) (*TargetFilter, error) {
	tf := &TargetFilter{
		Key:          cfg.MatchKey,
		ParentLogger: logger,
	}

	// Set the DoesMatch function for the filter
	switch cfg.Source {
	case config.FilterSource_Headers:
		tf.DoesMatch = tf.DoesMatch_Headers
	case config.FilterSource_Query:
		tf.DoesMatch = tf.DoesMatch_Query
	default:
		return nil, fmt.Errorf("error on filter: unhandled source (%s)", cfg.Source)
	}

	// Add anchors to the regex if they are not present
	// This speeds up skipping non-matching strings
	if !strings.HasPrefix(cfg.MatchRegex, "^") {
		cfg.MatchRegex = "^" + cfg.MatchRegex
	}
	if !strings.HasSuffix(cfg.MatchRegex, "$") {
		cfg.MatchRegex += "$"
	}

	// Compile the filter regex
	matcher, err := regexp.Compile(cfg.MatchRegex)
	if err != nil {
		return nil, err
	}
	tf.Search = matcher

	// Return the new target filter
	return tf, nil
}

// Check if a filter matches a request's headers
func (tf *TargetFilter) DoesMatch_Headers(r *http.Request) bool {
	value := r.Header.Get(tf.Key)
	return tf.Search.MatchString(value)
}

// Check if a filter matches a request's query parameters
func (tf *TargetFilter) DoesMatch_Query(r *http.Request) bool {
	value := r.URL.Query().Get(tf.Key)
	return tf.Search.MatchString(value)
}
