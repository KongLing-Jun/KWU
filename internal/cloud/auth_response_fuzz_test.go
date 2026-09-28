package cloud

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzDecodeCloudResponse(f *testing.F) {
	f.Add([]byte(`{"data":{"user_id":"usr_1"},"request_id":"req-1"}`), 0)
	f.Add([]byte(`{"error":{"code":"NETWORK_UNAVAILABLE","message":"offline","retryable":true},"request_id":"req-2"}`), 1)
	f.Add([]byte(`{"data":null}`), 0)
	f.Add([]byte(`{"data":`), 2)

	f.Fuzz(func(t *testing.T, payload []byte, statusClass int) {
		if len(payload) > cloudAuthResponseLimit+1 {
			t.Skip()
		}
		statusCodes := [...]int{200, 400, 500}
		statusIndex := statusClass % len(statusCodes)
		if statusIndex < 0 {
			statusIndex += len(statusCodes)
		}
		statusCode := statusCodes[statusIndex]

		var firstDestination json.RawMessage
		firstEnvelope, firstErr := decodeCloudResponse(payload, statusCode, &firstDestination)
		var secondDestination json.RawMessage
		secondEnvelope, secondErr := decodeCloudResponse(payload, statusCode, &secondDestination)

		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("decodeCloudResponse is nondeterministic: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr != nil {
			if firstErr.Error() != secondErr.Error() {
				t.Fatalf("decodeCloudResponse error changed: first=%q second=%q", firstErr, secondErr)
			}
			return
		}
		if statusCode != 200 {
			t.Fatalf("status %d succeeded", statusCode)
		}
		if firstEnvelope.Error != nil {
			t.Fatalf("successful envelope contains an error: %#v", firstEnvelope.Error)
		}
		if len(firstDestination) == 0 || bytes.Equal(firstDestination, []byte("null")) {
			t.Fatalf("successful response has empty data: %q", firstDestination)
		}
		if !reflect.DeepEqual(firstEnvelope, secondEnvelope) ||
			!bytes.Equal(firstDestination, secondDestination) {
			t.Fatal("decodeCloudResponse returned different results for the same input")
		}
	})
}
