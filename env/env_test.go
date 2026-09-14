// SPDX-License-Identifier: MIT

package env_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/umesh0492/go-libs/env"
)

func TestString(t *testing.T) {
	_ = os.Setenv("TEST_STR", "custom")
	defer func() { _ = os.Unsetenv("TEST_STR") }()

	assert.Equal(t, "custom", env.String("TEST_STR", "default"))
	assert.Equal(t, "default", env.String("TEST_UNSET_KEY", "default"))
}

func TestInt(t *testing.T) {
	_ = os.Setenv("TEST_INT", "8080")
	_ = os.Setenv("TEST_BAD_INT", "notanumber")
	defer func() {
		_ = os.Unsetenv("TEST_INT")
		_ = os.Unsetenv("TEST_BAD_INT")
	}()

	assert.Equal(t, 8080, env.Int("TEST_INT", 3000))
	assert.Equal(t, 3000, env.Int("TEST_UNSET_INT", 3000))
	assert.Equal(t, 3000, env.Int("TEST_BAD_INT", 3000))
}

func TestBool(t *testing.T) {
	_ = os.Setenv("TEST_BOOL_TRUE", "true")
	_ = os.Setenv("TEST_BOOL_FALSE", "0")
	_ = os.Setenv("TEST_BAD_BOOL", "invalid")
	defer func() {
		_ = os.Unsetenv("TEST_BOOL_TRUE")
		_ = os.Unsetenv("TEST_BOOL_FALSE")
		_ = os.Unsetenv("TEST_BAD_BOOL")
	}()

	assert.True(t, env.Bool("TEST_BOOL_TRUE", false))
	assert.False(t, env.Bool("TEST_BOOL_FALSE", true))
	assert.True(t, env.Bool("TEST_BAD_BOOL", true))
	assert.True(t, env.Bool("TEST_UNSET_BOOL", true))
}

func TestDuration(t *testing.T) {
	_ = os.Setenv("TEST_DUR", "5s")
	_ = os.Setenv("TEST_BAD_DUR", "notaduration")
	defer func() {
		_ = os.Unsetenv("TEST_DUR")
		_ = os.Unsetenv("TEST_BAD_DUR")
	}()

	assert.Equal(t, 5*time.Second, env.Duration("TEST_DUR", time.Minute))
	assert.Equal(t, time.Minute, env.Duration("TEST_BAD_DUR", time.Minute))
	assert.Equal(t, time.Minute, env.Duration("TEST_UNSET_DUR", time.Minute))
}

func TestMustString(t *testing.T) {
	_ = os.Setenv("TEST_MUST_STR", "present")
	defer func() { _ = os.Unsetenv("TEST_MUST_STR") }()

	assert.Equal(t, "present", env.MustString("TEST_MUST_STR"))
	assert.Panics(t, func() { env.MustString("TEST_MUST_STR_MISSING") })
}

func TestMustInt(t *testing.T) {
	_ = os.Setenv("TEST_MUST_INT", "42")
	_ = os.Setenv("TEST_MUST_BAD_INT", "notanumber")
	defer func() {
		_ = os.Unsetenv("TEST_MUST_INT")
		_ = os.Unsetenv("TEST_MUST_BAD_INT")
	}()

	assert.Equal(t, 42, env.MustInt("TEST_MUST_INT"))
	assert.Panics(t, func() { env.MustInt("TEST_MUST_INT_MISSING") })
	assert.Panics(t, func() { env.MustInt("TEST_MUST_BAD_INT") })
}
