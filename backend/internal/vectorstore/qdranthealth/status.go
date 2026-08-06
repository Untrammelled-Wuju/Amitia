// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

type State int

const (
	StateProcessNotStarted State = iota
	StateProcessStarted
	StateIdentityConfirmed
	StateLive
	StateReady
)

func (s State) String() string {
	switch s {
	case StateProcessNotStarted:
		return "process_not_started"
	case StateProcessStarted:
		return "process_started"
	case StateIdentityConfirmed:
		return "identity_confirmed"
	case StateLive:
		return "live"
	case StateReady:
		return "ready"
	}
	return "unknown"
}

func (s State) IsStarted() bool {
	return s >= StateProcessStarted
}

func (s State) IsIdentityConfirmed() bool {
	return s >= StateIdentityConfirmed
}

func (s State) IsLive() bool {
	return s >= StateLive
}

func (s State) IsReady() bool {
	return s >= StateReady
}

func (s State) AtLeast(other State) bool {
	return s >= other
}

func StateFromStrings(states ...string) []State {
	result := make([]State, 0, len(states))
	for _, s := range states {
		result = append(result, ParseState(s))
	}
	return result
}

func ParseState(s string) State {
	switch s {
	case "process_started":
		return StateProcessStarted
	case "identity_confirmed":
		return StateIdentityConfirmed
	case "live":
		return StateLive
	case "ready":
		return StateReady
	}
	return StateProcessNotStarted
}
