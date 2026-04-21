package schedule

import (
	"strings"
	"testing"
	"time"
)

func TestDecodeLastResults_Empty(t *testing.T) {
	got, err := decodeLastResults(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on empty input, got %+v", got)
	}
}

func TestDecodeLastResults_MultipleRunsReturnsLast(t *testing.T) {
	input := `[{"repo":{"Path":"/a","RelPath":"a"},"status":"OK","detail":"old","branch":"main"}]
[{"repo":{"Path":"/b","RelPath":"b"},"status":"UPDATED","detail":"new","branch":"main"}]
`
	got, err := decodeLastResults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Repo.RelPath != "b" || got[0].Status != "UPDATED" {
		t.Fatalf("expected second run, got %+v", got)
	}
}

func TestDecodeLastResults_TolleratesTrailingGarbage(t *testing.T) {
	input := `[{"repo":{"Path":"/a","RelPath":"a"},"status":"OK","detail":"","branch":"main"}]
not valid json`
	got, err := decodeLastResults(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Repo.RelPath != "a" {
		t.Fatalf("expected to recover first array, got %+v", got)
	}
}

func TestExpectedFireBefore_Daily(t *testing.T) {
	hour, minute := 8, 0
	info := &Info{Hour: &hour, Minute: &minute}

	started := time.Date(2026, 4, 20, 9, 15, 0, 0, time.UTC)
	got := ExpectedFireBefore(info, started)
	want := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expected %s, got %s", want, got)
	}

	started = time.Date(2026, 4, 20, 7, 0, 0, 0, time.UTC)
	got = ExpectedFireBefore(info, started)
	want = time.Date(2026, 4, 19, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestExpectedFireBefore_IntervalReturnsZero(t *testing.T) {
	interval := 3600
	info := &Info{IntervalSeconds: &interval}
	got := ExpectedFireBefore(info, time.Now())
	if !got.IsZero() {
		t.Errorf("expected zero time for interval schedule, got %s", got)
	}
}
