// SPDX-License-Identifier: MIT

// Package rbac provides a domain-agnostic, generic, wildcard-supporting Role-Based Access Control (RBAC) engine.
// It does not have any references to specific business domains or proprietary entities.
package rbac

import (
	"strings"
)

// Engine is a high-performance permission matching engine that supports wildcards.
type Engine struct{}

// NewEngine creates a new instance of the generic RBAC engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Match checks if a specific pattern (which may contain wildcards) permits the requested permission.
// Supported wildcard patterns:
// - "*" or "*:*" matches everything.
// - "resource:*" matches any action on that resource (e.g., "po:create").
// - "*:action" matches that action on any resource (e.g., "*:view").
// - Segment-based wildcards: "domain:resource:*" matches "domain:resource:create".
func (e *Engine) Match(pattern, requested string) bool {
	pattern = strings.TrimSpace(strings.ToLower(pattern))
	requested = strings.TrimSpace(strings.ToLower(requested))

	if pattern == "" || requested == "" {
		return false
	}

	if pattern == "*" || pattern == "*:*" {
		return true
	}

	if pattern == requested {
		return true
	}

	patternParts := strings.Split(pattern, ":")
	requestedParts := strings.Split(requested, ":")

	// If pattern has more segments than requested, it cannot match.
	if len(patternParts) > len(requestedParts) {
		return false
	}

	for i := 0; i < len(patternParts); i++ {
		pPart := patternParts[i]
		rPart := requestedParts[i]

		if pPart == "*" {
			// If it's the last part in the pattern, e.g., "po:*", and it's a wildcard,
			// it matches everything remaining in the requested permission.
			if i == len(patternParts)-1 {
				return true
			}
			continue
		}

		if pPart != rPart {
			return false
		}
	}

	// If we successfully matched all pattern parts, they must align in length
	// unless the last part of pattern was a wildcard (handled above).
	return len(patternParts) == len(requestedParts)
}

// HasPermission checks if the list of user permissions contains at least one pattern
// that matches the required permission.
func (e *Engine) HasPermission(userPerms []string, requiredPerm string) bool {
	for _, pattern := range userPerms {
		if e.Match(pattern, requiredPerm) {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if the user permissions match at least one of the required permissions.
func (e *Engine) HasAnyPermission(userPerms []string, requiredPerms ...string) bool {
	for _, req := range requiredPerms {
		if e.HasPermission(userPerms, req) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if the user permissions match all of the required permissions.
func (e *Engine) HasAllPermissions(userPerms []string, requiredPerms ...string) bool {
	for _, req := range requiredPerms {
		if !e.HasPermission(userPerms, req) {
			return false
		}
	}
	return true
}
