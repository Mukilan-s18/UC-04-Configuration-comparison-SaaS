package service

import "configguard/internal/domain"

// GetValidFixture returns the 2-row valid fixture defined in UC-04.
func GetValidFixture() []domain.UC04Record {
	return []domain.UC04Record{
		{
			RecordID:    "max_retries",
			Expected:    "3",
			Actual:      "5",
			Environment: "TEST",
		},
		{
			RecordID:    "region",
			Expected:    "us",
			Actual:      "us",
			Environment: "STAGE",
		},
	}
}

// GetInvalidFixture returns the 6-row invalid fixture defined in UC-04.
// Start with the two valid rows. Append four copies of the first row with fresh IDs X1 to X4,
// then apply one mutation per copy:
// 1. repeat max_retries (R1)
// 2. expected = empty string (R2)
// 3. actual = empty string (R3)
// 4. environment = PROD (R4)
func GetInvalidFixture() []domain.UC04Record {
	valid := GetValidFixture()

	c1 := domain.UC04Record{
		RecordID:    "max_retries", // Duplicate ID (R1)
		Expected:    "3",
		Actual:      "5",
		Environment: "TEST",
	}
	c2 := domain.UC04Record{
		RecordID:    "X2",
		Expected:    "", // Empty expected (R2)
		Actual:      "5",
		Environment: "TEST",
	}
	c3 := domain.UC04Record{
		RecordID:    "X3",
		Expected:    "3",
		Actual:      "", // Empty actual (R3)
		Environment: "TEST",
	}
	c4 := domain.UC04Record{
		RecordID:    "X4",
		Expected:    "3",
		Actual:      "5",
		Environment: "PROD", // Invalid env (R4)
	}

	return append(valid, c1, c2, c3, c4)
}
