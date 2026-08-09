package conformance

import (
	"fmt"
)

type Harness struct {
	protocolVersion string
	cases           []Case
}

func NewHarness(protocolVersion string) *Harness {
	return &Harness{
		protocolVersion: protocolVersion,
		cases:           make([]Case, 0),
	}
}

func (h *Harness) AddCase(c Case) {
	h.cases = append(h.cases, c)
}

func (h *Harness) AddCases(cases ...Case) {
	h.cases = append(h.cases, cases...)
}

func (h *Harness) Run() Result {
	result := NewResult(h.protocolVersion)
	for _, c := range h.cases {
		err := c.Validator.Validate(c.Input)
		passed := true
		if c.ExpectedValid && err != nil {
			passed = false
		} else if !c.ExpectedValid && err == nil {
			passed = false
			err = fmt.Errorf("expected validation to fail but it passed")
		}
		result.AddCaseResult(CaseResult{
			Name:   c.Name,
			Passed: passed,
			Err:    err,
		})
	}
	return result
}

func (h *Harness) RunAndReport() (Result, error) {
	result := h.Run()
	if !result.AllPassed() {
		return result, fmt.Errorf("%s", result.Error())
	}
	return result, nil
}

func (h *Harness) CaseCount() int {
	return len(h.cases)
}
