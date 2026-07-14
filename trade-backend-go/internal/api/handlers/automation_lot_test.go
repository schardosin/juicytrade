package handlers

import (
	"testing"

	"trade-backend-go/internal/automation/types"
)

// TestValidateLotConfig_Unset accepts an unset lot size (0 == single order).
func TestValidateLotConfig_Unset(t *testing.T) {
	tc := &types.TradeConfiguration{LotSize: 0}
	if err := validateLotConfig(tc); err != nil {
		t.Errorf("unset lot_size (0) must be accepted, got %v", err)
	}
}

// TestValidateLotConfig_Valid accepts lot sizes >= 1.
func TestValidateLotConfig_Valid(t *testing.T) {
	for _, ls := range []int{1, 2, 5, 100} {
		tc := &types.TradeConfiguration{LotSize: ls}
		if err := validateLotConfig(tc); err != nil {
			t.Errorf("lot_size %d must be accepted, got %v", ls, err)
		}
	}
}

// TestValidateLotConfig_Negative rejects negative lot sizes with a clear message (AC-1, FR-9).
func TestValidateLotConfig_Negative(t *testing.T) {
	for _, ls := range []int{-1, -5, -100} {
		tc := &types.TradeConfiguration{LotSize: ls}
		err := validateLotConfig(tc)
		if err == nil {
			t.Errorf("lot_size %d must be rejected", ls)
		}
	}
}
