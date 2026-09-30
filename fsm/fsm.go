// SPDX-License-Identifier: MIT

package fsm

import (
	"fmt"
)

// Transition defines a valid state change in the state machine.
type Transition struct {
	From   string
	To     string
	Action string
}

// Config holds the declarative transition rules and recognized states.
type Config struct {
	Transitions []Transition
	States      map[string]bool
}

// Machine represents the active instance of the Finite State Machine.
type Machine struct {
	config      Config
	transitions map[string]struct{}
}

func transitionKey(from, to, action string) string {
	return from + "\x00" + to + "\x00" + action
}

// NewMachine instantiates a new FSM with the given declarative transition configuration.
func NewMachine(transitions []Transition) *Machine {
	states := make(map[string]bool)
	tmap := make(map[string]struct{}, len(transitions))
	for _, t := range transitions {
		states[t.From] = true
		states[t.To] = true
		tmap[transitionKey(t.From, t.To, t.Action)] = struct{}{}
	}
	return &Machine{
		config: Config{
			Transitions: transitions,
			States:      states,
		},
		transitions: tmap,
	}
}

// CanTransition returns true if the transition from fromState to toState is allowed by action.
func (m *Machine) CanTransition(fromState, toState, action string) bool {
	_, ok := m.transitions[transitionKey(fromState, toState, action)]
	return ok
}

// ValidateTransition returns nil if the transition is valid, or a descriptive error otherwise.
func (m *Machine) ValidateTransition(fromState, toState, action string) error {
	if !m.config.States[fromState] {
		return fmt.Errorf("invalid source state: %q", fromState)
	}
	if !m.config.States[toState] {
		return fmt.Errorf("invalid destination state: %q", toState)
	}
	if m.CanTransition(fromState, toState, action) {
		return nil
	}
	return fmt.Errorf("illegal state transition from %q to %q via action %q", fromState, toState, action)
}
