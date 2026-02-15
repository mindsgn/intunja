package engine

import "testing"

func TestPercentZeroTotal(t *testing.T) {
	if got := percent(0, 0); got != 0 {
		t.Fatalf("expected 0 when total=0, got %v", got)
	}
}

func TestPercentNormal(t *testing.T) {
	if got := percent(50, 100); got != 50.00 {
		t.Fatalf("expected 50.00, got %v", got)
	}
}

func TestPercentPrecision(t *testing.T) {
	// 1 of 3 -> ~33.33 (two decimal places)
	if got := percent(1, 3); got < 33.33 || got > 33.34 {
		t.Fatalf("expected approx 33.33, got %v", got)
	}
}
