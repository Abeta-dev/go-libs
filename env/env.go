// SPDX-License-Identifier: MIT

// Package env provides zero-dependency typed environment variable retrieval
// with fallback defaults.
package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the string value of the environment variable key,
// or defaultVal if the variable is not set or empty.
func String(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// Int returns the integer value of the environment variable key,
// or defaultVal if the variable is unset, empty, or not a valid integer.
func Int(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

// Bool returns the boolean value of key, or defaultVal if unset or invalid.
// Accepts: "1", "t", "T", "true", "TRUE", "True", "0", "f", "F", "false", "FALSE", "False".
func Bool(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(val))
	if err != nil {
		return defaultVal
	}
	return parsed
}

// Duration returns the time.Duration parsed from key (e.g. "5s", "10m"),
// or defaultVal if unset, empty, or unparseable.
func Duration(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

// MustString returns the string value of key, or panics with a descriptive
// message if the variable is unset or empty.
func MustString(key string) string {
	val := os.Getenv(key)
	if val == "" {
		panic(fmt.Sprintf("env: required environment variable %q is not set", key))
	}
	return val
}

// MustInt returns the integer value of key, or panics if unset, empty, or invalid.
func MustInt(key string) int {
	val := MustString(key)
	parsed, err := strconv.Atoi(val)
	if err != nil {
		panic(fmt.Sprintf("env: environment variable %q must be a valid integer, got %q", key, val))
	}
	return parsed
}
