// SPDX-License-Identifier: MIT

// Package shutdown provides coordinated graceful teardown management for microservices,
// intercepting OS termination signals and executing registered cleanup hooks concurrently within a strict timeout budget.
package shutdown
