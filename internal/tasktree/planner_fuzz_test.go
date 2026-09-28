package tasktree

import (
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzParseTreeJSON(f *testing.F) {
	f.Add(`{"id":"tree-1","goal":"test","root_ids":["n1"],"status":"pending","nodes":{"n1":{"id":"n1","title":"Test","status":"pending"}}}`)
	f.Add("```json\n{\"tree\":{\"id\":\"wrapped\",\"nodes\":{}}}\n```")
	f.Add(`{"tree":`)
	f.Add("")

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 1<<20 {
			t.Skip()
		}

		first, firstErr := ParseTreeJSON(raw)
		second, secondErr := ParseTreeJSON(raw)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("ParseTreeJSON is nondeterministic: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			if firstErr.Error() != secondErr.Error() {
				t.Fatalf("ParseTreeJSON error changed: first=%q second=%q", firstErr, secondErr)
			}
			return
		}
		if first == nil || second == nil {
			t.Fatal("ParseTreeJSON returned a nil tree without an error")
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatal("ParseTreeJSON returned different trees for the same input")
		}
		if _, err := json.Marshal(first); err != nil {
			t.Fatalf("parsed tree cannot be encoded: %v", err)
		}
	})
}
