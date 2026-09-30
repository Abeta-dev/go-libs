// SPDX-License-Identifier: MIT

package uuidutil_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/umesh0492/go-libs/uuidutil"
)

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
