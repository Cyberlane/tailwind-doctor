package audit

import "testing"

func TestScannedExposurePerUnit(t *testing.T) {
	scanned := Scanned{Files: 2, ClassLists: 7, Utilities: 41}

	if got := scanned.exposure(ExposureUtility); got != 41 {
		t.Errorf("utility exposure = %d, want 41", got)
	}
	if got := scanned.exposure(ExposureClassList); got != 7 {
		t.Errorf("class-list exposure = %d, want 7", got)
	}
	if got := scanned.exposure(Exposure("nonsense")); got != 0 {
		t.Errorf("an unknown unit exposes nothing, got %d", got)
	}
}
