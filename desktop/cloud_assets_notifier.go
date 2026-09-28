package main

import (
	"sync"

	cloud "kwu/internal/cloud"
)

const cloudAssetsChangedEvent = "cloud-assets:changed"

// CloudAssetsChangedView is a wakeup payload, not an asset snapshot. The
// frontend always follows it by reading the current combined cache view.
type CloudAssetsChangedView struct {
	BalanceChanged      bool `json:"balanceChanged"`
	SubscriptionChanged bool `json:"subscriptionChanged"`
}

type cloudAssetsChangedSender func(CloudAssetsChangedView)

// cloudAssetsNotifier coalesces asset categories while one delivery is pending
// or in progress. Event-sync callers only merge dirty bits and signal a single
// buffered wakeup, so Wails availability cannot affect an applied event.
type cloudAssetsNotifier struct {
	mu      sync.Mutex
	dirty   CloudAssetsChangedView
	pending bool
	sender  cloudAssetsChangedSender
	wake    chan struct{}
	stop    chan struct{}
	done    chan struct{}
	started bool
	closed  bool
}

func newCloudAssetsNotifier() *cloudAssetsNotifier {
	return &cloudAssetsNotifier{
		wake: make(chan struct{}, 1),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

func (n *cloudAssetsNotifier) start() {
	if n == nil {
		return
	}
	n.mu.Lock()
	if n.started || n.closed {
		n.mu.Unlock()
		return
	}
	n.started = true
	n.mu.Unlock()
	go n.run()
}

func (n *cloudAssetsNotifier) bind(sender cloudAssetsChangedSender) {
	if n == nil {
		return
	}
	n.mu.Lock()
	if !n.closed {
		n.sender = sender
	}
	n.mu.Unlock()
}

func (n *cloudAssetsNotifier) notify(changes cloud.EventAssetChanges) {
	if n == nil {
		return
	}
	payload := CloudAssetsChangedView{
		BalanceChanged:      changes.BalanceChanged,
		SubscriptionChanged: changes.SubscriptionChanged,
	}
	if !payload.BalanceChanged && !payload.SubscriptionChanged {
		return
	}

	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.dirty.BalanceChanged = n.dirty.BalanceChanged || payload.BalanceChanged
	n.dirty.SubscriptionChanged = n.dirty.SubscriptionChanged || payload.SubscriptionChanged
	if n.pending {
		n.mu.Unlock()
		return
	}
	n.pending = true
	n.mu.Unlock()

	select {
	case n.wake <- struct{}{}:
	default:
	}
}

func (n *cloudAssetsNotifier) run() {
	defer close(n.done)
	for {
		select {
		case <-n.stop:
			return
		case <-n.wake:
			n.deliverPending()
		}
	}
}

func (n *cloudAssetsNotifier) deliverPending() {
	for {
		n.mu.Lock()
		if n.closed {
			n.dirty = CloudAssetsChangedView{}
			n.pending = false
			n.mu.Unlock()
			return
		}
		payload := n.dirty
		n.dirty = CloudAssetsChangedView{}
		sender := n.sender
		if !payload.BalanceChanged && !payload.SubscriptionChanged {
			n.pending = false
			n.mu.Unlock()
			return
		}
		n.mu.Unlock()

		if sender != nil {
			func() {
				defer func() {
					_ = recover()
				}()
				sender(payload)
			}()
		}

		n.mu.Lock()
		if !n.dirty.BalanceChanged && !n.dirty.SubscriptionChanged {
			n.pending = false
			n.mu.Unlock()
			return
		}
		n.mu.Unlock()
	}
}

func (n *cloudAssetsNotifier) close() {
	if n == nil {
		return
	}
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.closed = true
	n.sender = nil
	n.dirty = CloudAssetsChangedView{}
	n.pending = false
	started := n.started
	close(n.stop)
	n.mu.Unlock()
	if started {
		<-n.done
	}
}
