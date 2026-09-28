//go:build windows

package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/windows"
)

func startSecondInstanceSignal(uniqueID string, notify func()) (func(), error) {
	if notify == nil {
		return nil, errors.New("second-instance notification callback is unavailable")
	}
	sum := sha256.Sum256([]byte(uniqueID))
	name, err := windows.UTF16PtrFromString(fmt.Sprintf(`Local\kwu-second-instance-%x`, sum[:16]))
	if err != nil {
		return nil, err
	}
	event, createErr := windows.CreateEvent(nil, 0, 0, name)
	if errors.Is(createErr, windows.ERROR_ALREADY_EXISTS) {
		signalErr := windows.SetEvent(event)
		closeErr := windows.CloseHandle(event)
		if signalErr != nil {
			return nil, signalErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return nil, nil
	}
	if createErr != nil {
		return nil, createErr
	}

	var stopped atomic.Bool
	var closeOnce sync.Once
	go func() {
		defer windows.CloseHandle(event)
		for {
			_, waitErr := windows.WaitForSingleObject(event, windows.INFINITE)
			if waitErr != nil || stopped.Load() {
				return
			}
			notify()
		}
	}()
	return func() {
		closeOnce.Do(func() {
			stopped.Store(true)
			_ = windows.SetEvent(event)
		})
	}, nil
}
