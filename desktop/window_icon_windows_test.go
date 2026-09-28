//go:build windows

package main

import (
	"fmt"
	"reflect"
	"testing"
)

type fakeMainWindowIconAPI struct {
	window       uintptr
	dpi          uint32
	metrics      map[int]int
	metricDPIs   []uint32
	loadedSizes  [][2]int
	setIconCalls [][3]uintptr
	destroyed    []uintptr
	loadErrorAt  int
}

func (f *fakeMainWindowIconAPI) findWindow(className string, processID uint32) (uintptr, error) {
	if className != windowsMainWindowClass || processID != 42 {
		return 0, fmt.Errorf("unexpected window lookup: class=%q process=%d", className, processID)
	}
	return f.window, nil
}

func (f *fakeMainWindowIconAPI) windowDPI(uintptr) uint32 {
	return f.dpi
}

func (f *fakeMainWindowIconAPI) metricForDPI(index int, dpi uint32) int {
	f.metricDPIs = append(f.metricDPIs, dpi)
	return f.metrics[index]
}

func (f *fakeMainWindowIconAPI) loadIcon(resourceID, width, height int) (uintptr, error) {
	f.loadedSizes = append(f.loadedSizes, [2]int{width, height})
	if resourceID != wailsAppIconResourceID {
		return 0, fmt.Errorf("unexpected resource ID %d", resourceID)
	}
	if f.loadErrorAt == len(f.loadedSizes) {
		return 0, fmt.Errorf("load failed")
	}
	return uintptr(100 + len(f.loadedSizes)), nil
}

func (f *fakeMainWindowIconAPI) setIcon(window, iconType, icon uintptr) {
	f.setIconCalls = append(f.setIconCalls, [3]uintptr{window, iconType, icon})
}

func (f *fakeMainWindowIconAPI) destroyIcon(icon uintptr) {
	f.destroyed = append(f.destroyed, icon)
}

func (f *fakeMainWindowIconAPI) isWindow(uintptr) bool {
	return true
}

func TestApplyMainWindowIconsSetsSmallAndBigHandles(t *testing.T) {
	api := &fakeMainWindowIconAPI{
		window: 99,
		dpi:    144,
		metrics: map[int]int{
			smCXSmIcon: 20,
			smCYSmIcon: 20,
			smCXIcon:   40,
			smCYIcon:   40,
		},
	}
	window, dpi, icons, err := applyMainWindowIcons(api, windowsMainWindowClass, 42)
	if err != nil {
		t.Fatal(err)
	}
	if window != 99 || dpi != 144 {
		t.Fatalf("window, dpi = %d, %d; want 99, 144", window, dpi)
	}
	if icons != (mainWindowIconHandles{small: 101, big: 102}) {
		t.Fatalf("icons = %+v, want small=101 big=102", icons)
	}
	if want := []uint32{144, 144, 144, 144}; !reflect.DeepEqual(api.metricDPIs, want) {
		t.Fatalf("metric DPIs = %v, want %v", api.metricDPIs, want)
	}
	if want := [][2]int{{20, 20}, {40, 40}}; !reflect.DeepEqual(api.loadedSizes, want) {
		t.Fatalf("loaded icon sizes = %v, want %v", api.loadedSizes, want)
	}
	if want := [][3]uintptr{{99, iconSmall, 101}, {99, iconBig, 102}}; !reflect.DeepEqual(api.setIconCalls, want) {
		t.Fatalf("SetIcon calls = %v, want %v", api.setIconCalls, want)
	}
}

func TestApplyMainWindowIconsDoesNotPartiallyUpdateWhenLoadFails(t *testing.T) {
	api := &fakeMainWindowIconAPI{
		window:      99,
		dpi:         120,
		loadErrorAt: 2,
		metrics: map[int]int{
			smCXSmIcon: 16,
			smCYSmIcon: 16,
			smCXIcon:   32,
			smCYIcon:   32,
		},
	}
	if _, _, _, err := applyMainWindowIcons(api, windowsMainWindowClass, 42); err == nil {
		t.Fatal("expected big icon load failure")
	}
	if len(api.setIconCalls) != 0 {
		t.Fatalf("SetIcon called after incomplete load: %v", api.setIconCalls)
	}
	if want := []uintptr{101}; !reflect.DeepEqual(api.destroyed, want) {
		t.Fatalf("destroyed icons = %v, want %v", api.destroyed, want)
	}
}

func TestApplyMainWindowIconsFallsBackTo96DPI(t *testing.T) {
	api := &fakeMainWindowIconAPI{
		window: 99,
		metrics: map[int]int{
			smCXSmIcon: 16,
			smCYSmIcon: 16,
			smCXIcon:   32,
			smCYIcon:   32,
		},
	}
	_, dpi, _, err := applyMainWindowIcons(api, windowsMainWindowClass, 42)
	if err != nil {
		t.Fatal(err)
	}
	if dpi != 96 {
		t.Fatalf("dpi = %d, want 96", dpi)
	}
	if want := []uint32{96, 96, 96, 96}; !reflect.DeepEqual(api.metricDPIs, want) {
		t.Fatalf("metric DPIs = %v, want %v", api.metricDPIs, want)
	}
}

func TestReplaceMainWindowIconsReleasesPreviousOwnedHandles(t *testing.T) {
	api := &fakeMainWindowIconAPI{
		window: 99,
		dpi:    192,
		metrics: map[int]int{
			smCXSmIcon: 32,
			smCYSmIcon: 32,
			smCXIcon:   64,
			smCYIcon:   64,
		},
	}
	icons, dpi, err := replaceMainWindowIcons(api, 99, 99, mainWindowIconHandles{small: 71, big: 72})
	if err != nil {
		t.Fatal(err)
	}
	if dpi != 192 || icons != (mainWindowIconHandles{small: 101, big: 102}) {
		t.Fatalf("dpi, icons = %d, %+v", dpi, icons)
	}
	if want := []uintptr{71, 72}; !reflect.DeepEqual(api.destroyed, want) {
		t.Fatalf("destroyed icons = %v, want %v", api.destroyed, want)
	}
}
