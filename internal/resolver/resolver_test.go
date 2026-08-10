package resolver

import (
	"errors"
	"testing"
)

func TestResolveExact(t *testing.T) {
	items := []NamedID{
		{ID: 1, Name: "New"},
		{ID: 2, Name: "Closed"},
	}

	result := ResolveExact(items, "closed")
	if result.Err != nil {
		t.Fatalf("ResolveExact returned error: %v", result.Err)
	}
	if result.Item.ID != 2 {
		t.Fatalf("Item.ID = %d", result.Item.ID)
	}
}

func TestResolveExactUnknown(t *testing.T) {
	result := ResolveExact(nil, "Closed")
	if !errors.Is(result.Err, ErrUnknownName) {
		t.Fatalf("Err = %v, want ErrUnknownName", result.Err)
	}
}

func TestResolveExactAmbiguous(t *testing.T) {
	items := []NamedID{
		{ID: 1, Name: "Review"},
		{ID: 2, Name: "review"},
	}

	result := ResolveExact(items, "Review")
	if !errors.Is(result.Err, ErrAmbiguousName) {
		t.Fatalf("Err = %v, want ErrAmbiguousName", result.Err)
	}
	if len(result.Candidates) != 2 {
		t.Fatalf("Candidates = %d", len(result.Candidates))
	}
}
