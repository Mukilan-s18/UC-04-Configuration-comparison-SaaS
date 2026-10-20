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

	return domain.ValidationResult{}
}
