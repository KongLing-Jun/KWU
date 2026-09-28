package main

import "testing"

func TestDispatchRequestInitialize(t *testing.T) {
	result, rpcErr := dispatchRequest(request{Method: "initialize"})
	if rpcErr != nil || result == nil {
		t.Fatalf("dispatch initialize = (%v, %v), want result and nil error", result, rpcErr)
	}
}

func TestDispatchRequestUnknownMethod(t *testing.T) {
	result, rpcErr := dispatchRequest(request{Method: "missing/method"})
	if result != nil || rpcErr == nil || rpcErr.Code != codeMethodNotFound {
		t.Fatalf("dispatch unknown = (%v, %+v), want nil result and method-not-found", result, rpcErr)
	}
}
