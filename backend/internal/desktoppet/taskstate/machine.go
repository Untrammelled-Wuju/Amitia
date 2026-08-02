// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package taskstate

import (
	"github.com/u-ai/backend/internal/desktoppet/contracts"
)

type transitionEdge struct {
	from contracts.LifecycleStatus
	to   contracts.LifecycleStatus
}

var universalEdges = map[transitionEdge]bool{
	{contracts.StatusPending, contracts.StatusQueued}:                true,
	{contracts.StatusQueued, contracts.StatusProcessing}:             true,
	{contracts.StatusQueued, contracts.StatusCancelled}:              true,
	{contracts.StatusProcessing, contracts.StatusCancelling}:         true,
	{contracts.StatusProcessing, contracts.StatusSucceeded}:          true,
	{contracts.StatusProcessing, contracts.StatusPartiallySucceeded}: true,
	{contracts.StatusProcessing, contracts.StatusFailed}:             true,
	{contracts.StatusProcessing, contracts.StatusCancelled}:          true,
	{contracts.StatusProcessing, contracts.StatusQueued}:             true,
	{contracts.StatusCancelling, contracts.StatusCancelled}:          true,
	{contracts.StatusCancelling, contracts.StatusFailed}:             true,
	{contracts.StatusFailed, contracts.StatusQueued}:                 true,
	{contracts.StatusPartiallySucceeded, contracts.StatusQueued}:     true,
	{contracts.StatusCancelled, contracts.StatusQueued}:              true,
	{contracts.StatusSucceeded, contracts.StatusQueued}:              true,
}

var packageEdges = map[transitionEdge]bool{
	{contracts.StatusPending, contracts.StatusProcessing}:   true,
	{contracts.StatusPending, contracts.StatusCancelled}:    true,
	{contracts.StatusProcessing, contracts.StatusSucceeded}: true,
	{contracts.StatusProcessing, contracts.StatusFailed}:    true,
	{contracts.StatusProcessing, contracts.StatusCancelled}: true,
}

func IsLegalTransition(et contracts.EntityType, from, to contracts.LifecycleStatus) bool {
	if !contracts.IsAllowedStatusFor(et, from) {
		return false
	}
	if !contracts.IsAllowedStatusFor(et, to) {
		return false
	}
	if et == contracts.EntityPackage {
		return packageEdges[transitionEdge{from, to}]
	}
	return universalEdges[transitionEdge{from, to}]
}

func LegalTargets(et contracts.EntityType, from contracts.LifecycleStatus) []contracts.LifecycleStatus {
	allowed := contracts.AllowedStatusesFor(et)
	var targets []contracts.LifecycleStatus
	for _, to := range allowed {
		if IsLegalTransition(et, from, to) {
			targets = append(targets, to)
		}
	}
	return targets
}

func NeedsExecutionOwnership(from, to contracts.LifecycleStatus) bool {
	if from == contracts.StatusQueued && to == contracts.StatusProcessing {
		return true
	}
	if from == contracts.StatusProcessing && to.IsTerminal() {
		return true
	}
	if from == contracts.StatusCancelling && to == contracts.StatusCancelled {
		return true
	}
	if from == contracts.StatusProcessing && to == contracts.StatusCancelling {
		return false
	}
	return false
}

func IsRetryTransition(from, to contracts.LifecycleStatus) bool {
	if to != contracts.StatusQueued {
		return false
	}
	switch from {
	case contracts.StatusFailed, contracts.StatusPartiallySucceeded, contracts.StatusCancelled, contracts.StatusSucceeded:
		return true
	default:
		return false
	}
}

func IsCancelTransition(from, to contracts.LifecycleStatus) bool {
	switch {
	case from == contracts.StatusQueued && to == contracts.StatusCancelled:
		return true
	case from == contracts.StatusProcessing && to == contracts.StatusCancelling:
		return true
	case from == contracts.StatusCancelling && to == contracts.StatusCancelled:
		return true
	case from == contracts.StatusPending && to == contracts.StatusCancelled:
		return true
	default:
		return false
	}
}

func IsClaimTransition(from, to contracts.LifecycleStatus) bool {
	return from == contracts.StatusQueued && to == contracts.StatusProcessing
}

func IsFinalizeTransition(from, to contracts.LifecycleStatus) bool {
	if from != contracts.StatusProcessing {
		return false
	}
	switch to {
	case contracts.StatusSucceeded, contracts.StatusPartiallySucceeded, contracts.StatusFailed, contracts.StatusCancelled:
		return true
	default:
		return false
	}
}

var forbiddenTransitions = []transitionEdge{
	{contracts.StatusPending, contracts.StatusProcessing},
	{contracts.StatusQueued, contracts.StatusSucceeded},
	{contracts.StatusProcessing, contracts.StatusPending},
	{contracts.StatusCancelling, contracts.StatusProcessing},
	{contracts.StatusFailed, contracts.StatusSucceeded},
	{contracts.StatusCancelled, contracts.StatusSucceeded},
	{contracts.StatusSucceeded, contracts.StatusFailed},
}

func IsForbiddenTransition(from, to contracts.LifecycleStatus) bool {
	for _, e := range forbiddenTransitions {
		if e.from == from && e.to == to {
			return true
		}
	}
	if to == contracts.StatusProcessing {
		switch from {
		case contracts.StatusSucceeded, contracts.StatusPartiallySucceeded,
			contracts.StatusFailed, contracts.StatusCancelled:
			return true
		}
	}
	return false
}
