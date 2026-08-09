package tailwind

import "testing"

func TestAdapterForKnowsVersion3(t *testing.T) {
	adapter, found := AdapterFor(Version3)
	if !found || adapter.Version() != Version3 {
		t.Fatalf("v3 adapter = %T, found %v", adapter, found)
	}
}

func TestAdapterForRefusesUnknownVersions(t *testing.T) {
	if _, found := AdapterFor(VersionUnknown); found {
		t.Error("unknown version has an adapter")
	}
	if _, found := AdapterFor(Version("5")); found {
		t.Error("future version has an adapter")
	}
}
