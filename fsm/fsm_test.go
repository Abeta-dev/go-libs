// SPDX-License-Identifier: MIT

package fsm_test

import (
	"testing"

	"github.com/umesh0492/go-libs/fsm"
)

func TestFSM_ValidTransitions(t *testing.T) {
	machine := fsm.NewMachine([]fsm.Transition{
		{From: "DRAFT", To: "PENDING_APPROVAL", Action: "SUBMIT"},
		{From: "PENDING_APPROVAL", To: "APPROVED", Action: "APPROVE"},
		{From: "PENDING_APPROVAL", To: "REJECTED", Action: "REJECT"},
		{From: "REJECTED", To: "DRAFT", Action: "REVISE"},
	})

	if !machine.CanTransition("DRAFT", "PENDING_APPROVAL", "SUBMIT") {
		t.Fatal("expected DRAFT -> PENDING_APPROVAL to be permitted")
	}

	if err := machine.ValidateTransition("DRAFT", "PENDING_APPROVAL", "SUBMIT"); err != nil {
		t.Fatalf("expected nil error for valid transition, got: %v", err)
	}

	if err := machine.ValidateTransition("PENDING_APPROVAL", "APPROVED", "APPROVE"); err != nil {
		t.Fatalf("expected nil error for valid approval, got: %v", err)
	}
}

func TestFSM_InvalidTransitions(t *testing.T) {
	machine := fsm.NewMachine([]fsm.Transition{
		{From: "DRAFT", To: "ACTIVE", Action: "ACTIVATE"},
		{From: "ACTIVE", To: "ARCHIVED", Action: "ARCHIVE"},
	})

	// Direct skip from DRAFT to ARCHIVED
	if machine.CanTransition("DRAFT", "ARCHIVED", "ARCHIVE") {
		t.Fatal("expected DRAFT -> ARCHIVED to be prohibited")
	}

	err := machine.ValidateTransition("DRAFT", "ARCHIVED", "ARCHIVE")
	if err == nil {
		t.Fatal("expected error for illegal jump, got nil")
	}

	// Wrong action name
	if machine.CanTransition("DRAFT", "ACTIVE", "PUBLISH") {
		t.Fatal("expected wrong action name to be rejected")
	}

	// Unknown source state
	err = machine.ValidateTransition("UNKNOWN", "ACTIVE", "ACTIVATE")
	if err == nil {
		t.Fatal("expected error for unknown source state")
	}

	// Unknown destination state
	err = machine.ValidateTransition("DRAFT", "UNKNOWN", "ACTIVATE")
	if err == nil {
		t.Fatal("expected error for unknown destination state")
	}
}
