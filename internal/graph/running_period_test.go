package graph

import (
	"testing"
	"time"
)

func TestRunningPeriodRejectsInvalidSpanAndVerification(t *testing.T) {
	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	earlier := start.Add(-time.Hour)
	for _, node := range []Node{
		{ID: "bad-span", Type: RunningPeriod, Name: "Bad span", ModelVersion: 1, StartedAt: &start, EndedAt: &earlier},
		{ID: "bad-verification", Type: RunningPeriod, Name: "Bad verification", ModelVersion: 1, VerificationResult: "passed"},
		{ID: "wrong-type", Type: Machine, Name: "Wrong type", ModelVersion: 1, DeployedVersion: "1.0"},
	} {
		if err := New().AddNode(node); err == nil {
			t.Errorf("invalid node accepted: %+v", node)
		}
	}
}
