package ivx

import (
	"math"
	"testing"
	"time"
)

func TestCalculateDaysToExpirationAt(t *testing.T) {
	et, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load Eastern Time location: %v", err)
	}

	calc := NewCalculator()

	t.Run("0DTE morning 10:00 AM ET", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, et)
		expiration := "2025-06-15"

		dte, isExpired := calc.calculateDaysToExpirationAt(expiration, now)

		if isExpired {
			t.Error("expected isExpired = false, got true")
		}

		// 6 hours remaining until 4 PM market close → 6/24 = 0.25
		expected := 6.0 / 24.0
		if math.Abs(dte-expected) > 0.001 {
			t.Errorf("expected DTE ~%.4f, got %.4f", expected, dte)
		}
	})

	t.Run("0DTE midday 12:00 PM ET", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 12, 0, 0, 0, et)
		expiration := "2025-06-15"

		dte, isExpired := calc.calculateDaysToExpirationAt(expiration, now)

		if isExpired {
			t.Error("expected isExpired = false, got true")
		}

		// 4 hours remaining until 4 PM market close → 4/24 ≈ 0.1667
		expected := 4.0 / 24.0
		if math.Abs(dte-expected) > 0.001 {
			t.Errorf("expected DTE ~%.4f, got %.4f", expected, dte)
		}
	})

	t.Run("0DTE afternoon 3:00 PM ET", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 15, 0, 0, 0, et)
		expiration := "2025-06-15"

		dte, isExpired := calc.calculateDaysToExpirationAt(expiration, now)

		if isExpired {
			t.Error("expected isExpired = false, got true")
		}

		// 1 hour remaining until 4 PM market close → 1/24 ≈ 0.0417
		expected := 1.0 / 24.0
		if math.Abs(dte-expected) > 0.001 {
			t.Errorf("expected DTE ~%.4f, got %.4f", expected, dte)
		}
	})

	t.Run("0DTE after market close 5:00 PM ET", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 17, 0, 0, 0, et)
		expiration := "2025-06-15"

		dte, isExpired := calc.calculateDaysToExpirationAt(expiration, now)

		if !isExpired {
			t.Error("expected isExpired = true, got false")
		}

		if dte != 0.0001 {
			t.Errorf("expected DTE = 0.0001, got %.4f", dte)
		}
	})

	t.Run("1DTE expiration tomorrow morning", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, et)
		expiration := "2025-06-16"

		dte, isExpired := calc.calculateDaysToExpirationAt(expiration, now)

		if isExpired {
			t.Error("expected isExpired = false, got true")
		}

		// 1 full day + fractional day (6 hours remaining today → 6/24 = 0.25)
		// Total expected: 1.0 + 0.25 = 1.25
		expected := 1.25
		if math.Abs(dte-expected) > 0.001 {
			t.Errorf("expected DTE ~%.4f, got %.4f", expected, dte)
		}
	})

	t.Run("past expiration returns clamped value", func(t *testing.T) {
		now := time.Date(2025, 6, 15, 10, 0, 0, 0, et)
		expiration := "2025-06-14" // yesterday

		dte, isExpired := calc.calculateDaysToExpirationAt(expiration, now)

		if isExpired {
			t.Error("expected isExpired = false, got false (preserving existing behavior)")
		}

		// Past expiration gets clamped to 0.0001
		if dte != 0.0001 {
			t.Errorf("expected DTE = 0.0001, got %.4f", dte)
		}
	})
}
