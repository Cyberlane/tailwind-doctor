package audit

// Scanned records what the analysis was exposed to. These are the score's
// denominators, and they count resolved class lists only: an unresolvable
// className contributes to no denominator, because counting it would dilute
// measured debt in proportion to how much of a codebase cannot be read.
type Scanned struct {
	Files      int `json:"files"`
	ClassLists int `json:"classLists"`
	Utilities  int `json:"utilities"`
}

// exposure returns the denominator for one exposure unit. An unrecognised unit
// exposes nothing, which makes its rate zero rather than dividing by a number
// that means something else.
func (scanned Scanned) exposure(unit Exposure) int64 {
	switch unit {
	case ExposureUtility:
		return int64(scanned.Utilities)
	case ExposureClassList:
		return int64(scanned.ClassLists)
	}
	return 0
}
