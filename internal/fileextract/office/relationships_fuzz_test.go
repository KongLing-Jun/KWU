package office

import (
	"reflect"
	"testing"
)

func FuzzParseRelationships(f *testing.F) {
	f.Add([]byte(`<?xml version="1.0"?><Relationships><Relationship Id="rId1" Target="slides/slide1.xml"/></Relationships>`))
	f.Add([]byte(`<Relationships><Relationship Id="" Target="../outside.xml"/></Relationships>`))
	f.Add([]byte(`<Relationships><Relationship Id="rId1"`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}

		first, firstErr := parseRelationships(data)
		second, secondErr := parseRelationships(data)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("parseRelationships is nondeterministic: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			if firstErr.Error() != secondErr.Error() {
				t.Fatalf("parseRelationships error changed: first=%q second=%q", firstErr, secondErr)
			}
			return
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatal("parseRelationships returned different maps for the same input")
		}
		if len(first) > len(data)+1 {
			t.Fatalf("relationship count %d exceeds input-bound %d", len(first), len(data)+1)
		}
		for id, target := range first {
			if id == "" || target == "" {
				t.Fatalf("empty relationship escaped filtering: id=%q target=%q", id, target)
			}
		}
	})
}
