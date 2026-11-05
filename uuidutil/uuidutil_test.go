// SPDX-License-Identifier: MIT

package uuidutil_test

import (
	"crypto/rand"
	"errors"
	"testing"

	"github.com/Abeta-dev/go-libs/uuidutil"
	"github.com/google/uuid"
)

type failOnceReader struct {
	failed bool
}

func (r *failOnceReader) Read(p []byte) (int, error) {
	if !r.failed {
		r.failed = true
		return 0, errors.New("simulated CSPRNG failure")
	}
	return rand.Read(p)
}

func TestUUIDUtil_NewAndIsValid(t *testing.T) {
	id := uuidutil.New()
	if uuidutil.IsNil(id) {
		t.Fatal("expected non-nil UUID from New()")
	}

	str := uuidutil.NewString()
	if !uuidutil.IsValid(str) {
		t.Fatalf("expected valid UUID string from NewString(), got: %s", str)
	}

	if uuidutil.IsValid("not-a-valid-uuid") {
		t.Fatal("expected IsValid to return false for invalid string")
	}

	v7 := uuidutil.NewV7()
	if uuidutil.IsNil(v7) {
		t.Fatal("expected non-nil UUID from NewV7()")
	}
	if !uuidutil.IsValid(uuidutil.NewV7String()) {
		t.Fatal("expected valid UUID string from NewV7String()")
	}

	uuid.SetRand(&failOnceReader{})
	defer uuid.SetRand(nil)
	fallbackV7 := uuidutil.NewV7()
	if uuidutil.IsNil(fallbackV7) {
		t.Fatal("expected non-nil fallback UUID from NewV7() on CSPRNG error")
	}
}

func TestUUIDUtil_Parse(t *testing.T) {
	raw := uuid.New().String()

	parsed, err := uuidutil.Parse(raw)
	if err != nil {
		t.Fatalf("unexpected error parsing valid UUID: %v", err)
	}
	if parsed.String() != raw {
		t.Fatalf("expected parsed %s, got %s", raw, parsed.String())
	}

	_, err = uuidutil.Parse("invalid")
	if err == nil {
		t.Fatal("expected error parsing invalid UUID string")
	}
}

func TestUUIDUtil_ParsePtr(t *testing.T) {
	ptr, err := uuidutil.ParsePtr("")
	if err != nil || ptr != nil {
		t.Fatalf("expected nil ptr and nil error for empty string, got ptr: %v, err: %v", ptr, err)
	}

	raw := uuid.New().String()
	ptr, err = uuidutil.ParsePtr(raw)
	if err != nil || ptr == nil {
		t.Fatalf("unexpected error parsing valid UUID ptr: %v", err)
	}
	if ptr.String() != raw {
		t.Fatalf("expected %s, got %s", raw, ptr.String())
	}

	ptr, err = uuidutil.ParsePtr("invalid-uuid")
	if err == nil || ptr != nil {
		t.Fatalf("expected error and nil ptr for invalid UUID, got ptr: %v, err: %v", ptr, err)
	}
}

func TestUUIDUtil_MustParse(t *testing.T) {
	raw := uuid.New().String()
	id := uuidutil.MustParse(raw)
	if id.String() != raw {
		t.Fatalf("expected %s, got %s", raw, id.String())
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from MustParse with invalid input")
		}
	}()
	uuidutil.MustParse("invalid")
}
