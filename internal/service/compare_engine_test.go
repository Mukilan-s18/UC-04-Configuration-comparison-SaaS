package service

import (
	"testing"

	"configguard/internal/domain"
)

func TestCompareEngine_ValidFixture(t *testing.T) {
	engine := NewCompareEngine()
	records := GetValidFixture()

	// 1. Validate
	valRes := engine.ValidateBatch(records)
	if valRes.ValidRows != 2 {
		t.Fatalf("expected valid_rows 2, got %d", valRes.ValidRows)
	}
	if valRes.InvalidRows != 0 {
		t.Fatalf("expected invalid_rows 0, got %d", valRes.InvalidRows)
	}
	if len(valRes.Issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(valRes.Issues))
	}

	// 2. Process
	procRes := engine.ProcessBatch(records)
	if procRes.Summary.InputRows != 2 {
		t.Errorf("expected input_rows 2, got %d", procRes.Summary.InputRows)
	}
	if procRes.Summary.ChangedRows != 1 {
		t.Errorf("expected changed_rows 1, got %d", procRes.Summary.ChangedRows)
	}
	if procRes.Summary.UnchangedRows != 1 {
		t.Errorf("expected unchanged_rows 1, got %d", procRes.Summary.UnchangedRows)
	}

	if len(procRes.Rows) != 2 {
		t.Fatalf("expected 2 output rows, got %d", len(procRes.Rows))
	}
	if procRes.Rows[0].RecordID != "max_retries" || procRes.Rows[0].Comparison != "CHANGED" {
		t.Errorf("row 0 expected max_retries CHANGED, got %+v", procRes.Rows[0])
	}
	if procRes.Rows[1].RecordID != "region" || procRes.Rows[1].Comparison != "UNCHANGED" {
		t.Errorf("row 1 expected region UNCHANGED, got %+v", procRes.Rows[1])
	}
}

func TestCompareEngine_InvalidFixture(t *testing.T) {
	engine := NewCompareEngine()
	records := GetInvalidFixture()

	valRes := engine.ValidateBatch(records)
	if valRes.ValidRows != 2 {
		t.Errorf("expected valid_rows 2, got %d", valRes.ValidRows)
	}
	if valRes.InvalidRows != 4 {
		t.Errorf("expected invalid_rows 4, got %d", valRes.InvalidRows)
	}
	if len(valRes.Issues) != 4 {
		t.Fatalf("expected exactly 4 issues, got %d: %+v", len(valRes.Issues), valRes.Issues)
	}

	ruleMap := make(map[string]domain.Issue)
	for _, issue := range valRes.Issues {
		ruleMap[issue.RuleID] = issue
	}

	if _, ok := ruleMap["R1"]; !ok {
		t.Errorf("missing R1 issue: %+v", valRes.Issues)
	}
	if _, ok := ruleMap["R2"]; !ok {
		t.Errorf("missing R2 issue: %+v", valRes.Issues)
	}
	if _, ok := ruleMap["R3"]; !ok {
		t.Errorf("missing R3 issue: %+v", valRes.Issues)
	}
	if _, ok := ruleMap["R4"]; !ok {
		t.Errorf("missing R4 issue: %+v", valRes.Issues)
	}
}
