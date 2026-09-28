//go:build windows

package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestWindowsSecondInstanceSignalNotifiesOriginalProcess(t *testing.T) {
	uniqueID := fmt.Sprintf("test-%d-%d", os.Getpid(), time.Now().UnixNano())
	notified := make(chan struct{}, 1)
	closeFirst, err := startSecondInstanceSignal(uniqueID, func() {
		notified <- struct{}{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if closeFirst == nil {
		t.Fatal("first instance did not retain the signal listener")
	}
	t.Cleanup(closeFirst)

	closeSecond, err := startSecondInstanceSignal(uniqueID, func() {
		t.Error("second instance callback must not be retained")
	})
	if err != nil {
		t.Fatal(err)
	}
	if closeSecond != nil {
		closeSecond()
		t.Fatal("second instance unexpectedly retained a signal listener")
	}

	select {
	case <-notified:
	case <-time.After(time.Second):
		t.Fatal("original instance did not receive the second-instance signal")
	}
}
