//go:build !testdiag

package main

import "context"

func prepareWorkBuddyRCTest(context.Context, *App) error { return nil }

func workBuddyRCTestMode() bool { return false }
