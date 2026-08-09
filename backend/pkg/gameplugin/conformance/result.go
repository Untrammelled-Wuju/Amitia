package conformance

import "fmt"

type CaseResult struct {
	Name    string
	Passed  bool
	Err     error
}

func (r CaseResult) Error() string {
	if r.Err != nil {
		return fmt.Sprintf("case %q failed: %v", r.Name, r.Err)
	}
	return fmt.Sprintf("case %q: no error details", r.Name)
}

type Result struct {
	ProtocolVersion string
	Passed          int
	Failed          int
	Results         []CaseResult
}

func NewResult(protocolVersion string) Result {
	return Result{
		ProtocolVersion: protocolVersion,
		Results:         make([]CaseResult, 0),
	}
}

func (r *Result) AddCaseResult(cr CaseResult) {
	r.Results = append(r.Results, cr)
	if cr.Passed {
		r.Passed++
	} else {
		r.Failed++
	}
}

func (r Result) AllPassed() bool {
	return r.Failed == 0
}

func (r Result) Total() int {
	return r.Passed + r.Failed
}

func (r Result) Error() string {
	if r.AllPassed() {
		return fmt.Sprintf("conformance suite passed: %d/%d", r.Passed, r.Total())
	}
	return fmt.Sprintf("conformance suite failed: %d/%d passed, %d failed", r.Passed, r.Total(), r.Failed)
}
