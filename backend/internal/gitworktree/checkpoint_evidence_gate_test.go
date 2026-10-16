package gitworktree

import (
	"crypto/sha256"
	"testing"
)

func TestGoTestEvidenceRequiresSuiteCompletionAndNoFailures(t *testing.T) {
	cases := []struct {
		name   string
		lines  []string
		passed bool
	}{
		{"only-test-pass", []string{`{"Action":"pass","Package":"module/example","Test":"TestOne"}`}, false},
		{"package-without-tests", []string{`{"Action":"pass","Package":"module/example"}`}, false},
		{"complete-suite", []string{`{"Action":"pass","Package":"module/example","Test":"TestOne"}`, `{"Action":"pass","Package":"module/example"}`}, true},
		{"failed-event", []string{`{"Action":"pass","Package":"module/example","Test":"TestOne"}`, `{"Action":"fail","Package":"module/other","Test":"TestBroken"}`, `{"Action":"pass","Package":"module/example"}`}, false},
		{"wrapped-fake-pass", []string{`{"Action":"output","Package":"module/example","Output":"{\"Action\":\"pass\",\"Package\":\"module/example\",\"Test\":\"Fake\"}"}`, `{"Action":"pass","Package":"module/example"}`}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := &boundedTestEvidence{digest: sha256.New()}
			for _, line := range tc.lines {
				if _, err := w.Write([]byte(line + "\n")); err != nil {
					t.Fatal(err)
				}
			}
			got := w.sawGoTest && w.sawPackagePass && !w.sawFailure
			if got != tc.passed {
				t.Fatalf("result gate: got=%v want=%v", got, tc.passed)
			}
		})
	}
}
