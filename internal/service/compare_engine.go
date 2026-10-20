package service

import (
	"strings"

	"configguard/internal/domain"
)

// CompareEngine implements validation rules R1-R4 and the pure comparison processor for UC-04.
type CompareEngine struct{}

func NewCompareEngine() *CompareEngine {
	return &CompareEngine{}
}

// ValidateBatch evaluates a batch of UC04 records against rules R1 to R4.
// It reports every failing rule, but counts each invalid row once.
// Only later occurrences of an ID are duplicates.
func (e *CompareEngine) ValidateBatch(records []domain.UC04Record) domain.ValidationResult {
	var issues []domain.Issue
	seenIDs := make(map[string]bool)
	invalidRowsMap := make(map[int]bool)

	for i, rawRec := range records {
		rowIndex := i + 1 // 1-based index
		rowHasError := false

		// Trim strings
		recID := strings.TrimSpace(rawRec.RecordID)
		expected := strings.TrimSpace(rawRec.Expected)
		actual := strings.TrimSpace(rawRec.Actual)
		env := strings.TrimSpace(rawRec.Environment)

		// R1: record_id is a nonempty string of at most 20 characters and is unique within the batch.
		// Note: rawRec.RecordID is case-sensitive.
		if rawRec.RecordID == "" || len(recID) == 0 || len(rawRec.RecordID) > 20 {
			issues = append(issues, domain.Issue{
				RowIndex: rowIndex,
				Field:    "record_id",
				RuleID:   "R1",
			})
			rowHasError = true
		} else if seenIDs[rawRec.RecordID] {
			// Duplicate ID (only later occurrences are duplicates)
			issues = append(issues, domain.Issue{
				RowIndex: rowIndex,
				Field:    "record_id",
				RuleID:   "R1",
			})
			rowHasError = true
		} else {
			seenIDs[rawRec.RecordID] = true
		}

		// R2: expected is a string of 1 to 40 characters after trimming spaces.
		if len(expected) < 1 || len(expected) > 40 {
			issues = append(issues, domain.Issue{
				RowIndex: rowIndex,
				Field:    "expected",
				RuleID:   "R2",
			})
			rowHasError = true
		}

		// R3: actual is a string of 1 to 40 characters after trimming spaces.
		if len(actual) < 1 || len(actual) > 40 {
			issues = append(issues, domain.Issue{
				RowIndex: rowIndex,
				Field:    "actual",
				RuleID:   "R3",
			})
			rowHasError = true
		}

		// R4: environment is TEST or STAGE.
		if env != "TEST" && env != "STAGE" {
			issues = append(issues, domain.Issue{
				RowIndex: rowIndex,
				Field:    "environment",
				RuleID:   "R4",
			})
			rowHasError = true
		}

		if rowHasError {
			invalidRowsMap[rowIndex] = true
		}
	}

	totalRows := len(records)
	invalidCount := len(invalidRowsMap)
	validCount := totalRows - invalidCount

	if issues == nil {
		issues = []domain.Issue{}
	}

	return domain.ValidationResult{
		ValidRows:   validCount,
		InvalidRows: invalidCount,
		Issues:      issues,
	}
}

// ProcessBatch compares trimmed expected and actual strings using case-sensitive equality.
// Assumes validation passed (or call ValidateBatch first).
// Preserves row order in output.
func (e *CompareEngine) ProcessBatch(records []domain.UC04Record) domain.ProcessResult {
	var processedRows []domain.ProcessedRow
	changedCount := 0
	unchangedCount := 0

	for _, rec := range records {
		trimmedExpected := strings.TrimSpace(rec.Expected)
		trimmedActual := strings.TrimSpace(rec.Actual)

		var comparison string
		if trimmedExpected == trimmedActual {
			comparison = "UNCHANGED"
			unchangedCount++
		} else {
			comparison = "CHANGED"
			changedCount++
		}

		processedRows = append(processedRows, domain.ProcessedRow{
			RecordID:    rec.RecordID,
			Expected:    rec.Expected,
			Actual:      rec.Actual,
			Environment: rec.Environment,
			Comparison:  comparison,
		})
	}

	if processedRows == nil {
		processedRows = []domain.ProcessedRow{}
	}

	return domain.ProcessResult{
		Summary: domain.ProcessSummary{
			InputRows:     len(records),
			ChangedRows:   changedCount,
			UnchangedRows: unchangedCount,
		},
		Rows:   processedRows,
		Issues: []domain.Issue{},
	}
}
