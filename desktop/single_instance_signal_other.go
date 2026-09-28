//go:build !windows

package main

func startSecondInstanceSignal(_ string, _ func()) (func(), error) {
	return nil, nil
}
