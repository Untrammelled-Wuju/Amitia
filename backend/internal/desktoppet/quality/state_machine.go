// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"fmt"
)

type EvaluationStateTransition struct {
	From EvaluationExecutionStatus
	To   EvaluationExecutionStatus
}

var allowedTransitions = map[EvaluationStateTransition]bool{
	{EvalCreated, EvalQueued}:       true,
	{EvalCreated, EvalPending}:      true,
	{EvalQueued, EvalPending}:       true,
	{EvalPending, EvalRunning}:      true,
	{EvalRunning, EvalCommitting}:   true,
	{EvalCommitting, EvalSucceeded}: true,
	{EvalCommitting, EvalFailed}:    true,
	{EvalRunning, EvalFailed}:       true,
	{EvalPending, EvalFailed}:       true,
	{EvalQueued, EvalFailed}:        true,
	{EvalPending, EvalCancelled}:    true,
	{EvalRunning, EvalCancelled}:    true,
	{EvalPending, EvalSuperseded}:   true,
	{EvalQueued, EvalSuperseded}:    true,
}

func CanTransition(from, to EvaluationExecutionStatus) bool {
	if from == to {
		return true
	}
	return allowedTransitions[EvaluationStateTransition{From: from, To: to}]
}

func MustTransition(from, to EvaluationExecutionStatus) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("invalid evaluation state transition: %s -> %s", from, to)
	}
	return nil
}
