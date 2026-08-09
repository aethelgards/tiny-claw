package tools

import (
	"testing"
	"time"
)

// TestLockManagerDisjointPaths verifies that exclusive acquisitions of two
// different paths do not block each other: the per-path granularity is what
// lets 1.txt and 2.txt be updated concurrently.
func TestLockManagerDisjointPaths(t *testing.T) {
	m := newLockManager()
	releaseA := m.acquirePath("/a", true)
	releaseB := m.acquirePath("/b", true)
	releaseB()
	releaseA()
}

// TestLockManagerSamePathExcludes verifies that a second exclusive acquisition
// of an already-locked path blocks until the first holder releases.
func TestLockManagerSamePathExcludes(t *testing.T) {
	m := newLockManager()
	release1 := m.acquirePath("/a", true)

	acquired := make(chan struct{})
	go func() {
		release2 := m.acquirePath("/a", true)
		close(acquired)
		release2()
	}()

	select {
	case <-acquired:
		t.Fatal("second exclusive acquisition on same path should block")
	case <-time.After(50 * time.Millisecond):
	}
	release1()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second exclusive acquisition did not proceed after release")
	}
}

// TestLockManagerReadShare verifies that shared acquisitions of the same path
// overlap, and that an exclusive acquisition waits for all readers.
func TestLockManagerReadShare(t *testing.T) {
	m := newLockManager()
	releaseR1 := m.acquirePath("/a", false)
	releaseR2 := m.acquirePath("/a", false)

	acquired := make(chan struct{})
	go func() {
		releaseW := m.acquirePath("/a", true)
		close(acquired)
		releaseW()
	}()

	select {
	case <-acquired:
		t.Fatal("exclusive acquisition should block while readers hold")
	case <-time.After(50 * time.Millisecond):
	}
	releaseR1()
	releaseR2()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("exclusive acquisition did not proceed after readers released")
	}
}

// TestLockManagerGlobalExcludes verifies the global tier (bash) excludes every
// per-path acquisition and is itself excluded by one.
func TestLockManagerGlobalExcludes(t *testing.T) {
	m := newLockManager()
	releaseG := m.acquireGlobal()

	acquired := make(chan struct{})
	go func() {
		release := m.acquirePath("/a", false)
		close(acquired)
		release()
	}()

	select {
	case <-acquired:
		t.Fatal("per-path acquisition should block while global lock held")
	case <-time.After(50 * time.Millisecond):
	}
	releaseG()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("per-path acquisition did not proceed after global release")
	}

	releaseR := m.acquirePath("/a", false)
	acquiredG := make(chan struct{})
	go func() {
		release := m.acquireGlobal()
		close(acquiredG)
		release()
	}()

	select {
	case <-acquiredG:
		t.Fatal("global acquisition should block while per-path lock held")
	case <-time.After(50 * time.Millisecond):
	}
	releaseR()
	select {
	case <-acquiredG:
	case <-time.After(2 * time.Second):
		t.Fatal("global acquisition did not proceed after per-path release")
	}
}

// TestLockManagerEntryCleanup verifies the refcount drops the map entry once
// the last holder releases, so a long-running process never accumulates one
// entry per touched path.
func TestLockManagerEntryCleanup(t *testing.T) {
	m := newLockManager()
	m.acquirePath("/a", true)()

	m.mu.Lock()
	_, ok := m.paths["/a"]
	m.mu.Unlock()
	if ok {
		t.Fatal("path entry should be removed after last release")
	}
}
