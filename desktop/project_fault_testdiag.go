//go:build testdiag

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	workBuddyRCFaultPointEnv = "WORKBUDDY_RC_FAULT_POINT"
	workBuddyRCBarrierEnv    = "WORKBUDDY_RC_BARRIER_FILE"
)

// workBuddyProjectFaultPoint exists only in the output-only testdiag build. It
// publishes a durable, content-free barrier and waits for the E2E owner to kill
// the exact Desktop process tree.
func workBuddyProjectFaultPoint(point, idempotencyKey string) {
	if os.Getenv(workBuddyRCFaultPointEnv) != point {
		return
	}
	barrier := filepath.Clean(os.Getenv(workBuddyRCBarrierEnv))
	if !filepath.IsAbs(barrier) {
		panic(fmt.Sprintf("%s must be absolute", workBuddyRCBarrierEnv))
	}
	if err := os.MkdirAll(filepath.Dir(barrier), 0o700); err != nil {
		panic(err)
	}
	keyHash := sha256.Sum256([]byte(idempotencyKey))
	payload, err := json.Marshal(map[string]any{
		"schema":                 "reasonix.workbuddy-rc-fault/v1",
		"point":                  point,
		"pid":                    os.Getpid(),
		"idempotency_key_sha256": fmt.Sprintf("%x", keyHash[:]),
		"time":                   time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		panic(err)
	}
	file, err := os.OpenFile(barrier, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		panic(err)
	}
	if _, err = file.Write(append(payload, '\n')); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		panic(err)
	}
	select {}
}
