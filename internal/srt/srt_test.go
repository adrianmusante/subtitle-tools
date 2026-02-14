package srt

import "testing"

func TestValidateSequentialIdx_OK(t *testing.T) {
	subs := []*Subtitle{{Idx: 1}, {Idx: 2}, {Idx: 3}}
	if err := ValidateSequentialIdx(subs); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateSequentialIdx_StartsAtOne(t *testing.T) {
	subs := []*Subtitle{{Idx: 2}, {Idx: 3}}
	if err := ValidateSequentialIdx(subs); err == nil {
		t.Fatalf("expected error for non-1 start")
	}
}

func TestValidateSequentialIdx_Gap(t *testing.T) {
	subs := []*Subtitle{{Idx: 1}, {Idx: 3}}
	if err := ValidateSequentialIdx(subs); err == nil {
		t.Fatalf("expected error for gap")
	}
}

func TestReindex(t *testing.T) {
	subs := []*Subtitle{{Idx: 10}, {Idx: 20}, {Idx: 30}}
	Reindex(subs)
	if subs[0].Idx != 1 || subs[1].Idx != 2 || subs[2].Idx != 3 {
		t.Fatalf("unexpected indexes after reindex: %d, %d, %d", subs[0].Idx, subs[1].Idx, subs[2].Idx)
	}
}
