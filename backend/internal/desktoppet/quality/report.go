// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type reportInput struct {
	MeasurementSetID string `json:"measurementSetId"`
	RevisionHash     string `json:"revisionHash"`
	ProfileHash      string `json:"profileHash"`
	EngineVersion    string `json:"engineVersion"`
}

type reportExecution struct {
	Status     EvaluationExecutionStatus `json:"status"`
	DurationMs int64                     `json:"durationMs"`
}

type qualityReport struct {
	SchemaVersion     int              `json:"schemaVersion"`
	EvaluationID     string           `json:"evaluationId"`
	ActionRevisionID string           `json:"actionRevisionId"`
	ActionKey        string           `json:"actionKey"`
	Input            reportInput      `json:"input"`
	Execution        reportExecution  `json:"execution"`
	Verdict          string           `json:"verdict"`
	OverallScore     *float64         `json:"overallScore,omitempty"`
	OverallConfidence float64         `json:"overallConfidence"`
	DimensionScores  []DimensionScore `json:"dimensionScores"`
	Findings         []QualityFinding `json:"findings"`
}

type ReportGenerator struct {
	dataDir string
}

func NewReportGenerator(dataDir string) *ReportGenerator {
	return &ReportGenerator{dataDir: dataDir}
}

func (g *ReportGenerator) GenerateReport(eval *QualityEvaluation, result *ActionQualityResult, findings []QualityFinding, scores []DimensionScore, observations []Observation) (reportPath string, reportHash string, err error) {
	if eval == nil {
		return "", "", fmt.Errorf("evaluation is nil")
	}

	reportDir := filepath.Join(g.dataDir, "desktop-pets", "quality-reports", eval.ID)
	if err = os.MkdirAll(reportDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create report dir: %w", err)
	}

	verdict := eval.Verdict
	if result != nil && result.Verdict != nil {
		verdict = string(*result.Verdict)
	}

	overallScore := eval.OverallScore
	if result != nil && result.OverallScore != nil {
		overallScore = result.OverallScore
	}

	overallConfidence := eval.OverallConfidence
	if result != nil {
		overallConfidence = result.OverallConfidence
	}

	dimensionScores := scores
	if len(dimensionScores) == 0 && result != nil {
		dimensionScores = result.DimensionScores
	}

	if findings == nil {
		findings = []QualityFinding{}
	}
	if dimensionScores == nil {
		dimensionScores = []DimensionScore{}
	}

	report := qualityReport{
		SchemaVersion:      1,
		EvaluationID:       eval.ID,
		ActionRevisionID:   eval.ActionRevisionID,
		ActionKey:          eval.ActionKey,
		Input: reportInput{
			MeasurementSetID: eval.MeasurementSetID,
			ProfileHash:      eval.ProfileHash,
			EngineVersion:     eval.EngineVersion,
		},
		Execution: reportExecution{
			Status:     eval.ExecutionStatus,
			DurationMs: computeDurationMs(eval.StartedAt, eval.CompletedAt),
		},
		Verdict:           verdict,
		OverallScore:      overallScore,
		OverallConfidence: overallConfidence,
		DimensionScores:   dimensionScores,
		Findings:          findings,
	}

	data, err := json.Marshal(report)
	if err != nil {
		return "", "", fmt.Errorf("marshal report: %w", err)
	}

	finalPath := filepath.Join(reportDir, "quality-report.json")
	tmpPath := finalPath + ".tmp"

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("create temp report: %w", err)
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("write temp report: %w", err)
	}
	if err = f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("fsync temp report: %w", err)
	}
	if err = f.Close(); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("close temp report: %w", err)
	}

	verifyData, err := os.ReadFile(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("read temp report for verify: %w", err)
	}

	hash := sha256.Sum256(verifyData)
	reportHash = hex.EncodeToString(hash[:])

	if err = os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", "", fmt.Errorf("rename temp report: %w", err)
	}

	return finalPath, reportHash, nil
}

func computeDurationMs(startedAt, completedAt string) int64 {
	if startedAt == "" || completedAt == "" {
		return 0
	}
	start, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.RFC3339, completedAt)
	if err != nil {
		return 0
	}
	d := end.Sub(start)
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}
