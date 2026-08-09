package tools

import "sync"

// lockManager provides per-resource read-write locks so operations on
// disjoint paths (e.g. writing 1.txt and 2.txt) proceed concurrently,
// while operations on the same path are serialized. A global tier exists
// for bash, whose command may touch any file and therefore cannot be
// keyed by a single path.
type lockManager struct {
	mu     sync.Mutex
	global sync.RWMutex
	paths  map[string]*pathLock
}

// pathLock is a per-path RWMutex with a reference count. refs counts every
// goroutine holding or waiting on rw, so an entry is only removed from the
// map when no goroutine can still touch it.
type pathLock struct {
	rw   sync.RWMutex
	refs int
}

// toolLocks is the process-wide lock manager shared by every tool instance.
// Package-level (not per-tool) so a read_file on one instance excludes a
// write_file on another instance touching the same path.
var toolLocks = newLockManager()

func newLockManager() *lockManager {
	return &lockManager{paths: make(map[string]*pathLock)}
}

// acquirePath takes the shared global lock (excluding bash) then the lock
// for the specific resource path: exclusive when write is true, shared
// otherwise. The returned func releases both locks in reverse order.
func (m *lockManager) acquirePath(path string, write bool) func() {
	m.global.RLock()

	m.mu.Lock()
	pl := m.paths[path]
	if pl == nil {
		pl = &pathLock{}
		m.paths[path] = pl
	}
	pl.refs++
	m.mu.Unlock()

	if write {
		pl.rw.Lock()
	} else {
		pl.rw.RLock()
	}

	return func() {
		if write {
			pl.rw.Unlock()
		} else {
			pl.rw.RUnlock()
		}
		m.mu.Lock()
		pl.refs--
		if pl.refs == 0 {
			delete(m.paths, path)
		}
		m.mu.Unlock()
		m.global.RUnlock()
	}
}

// acquireGlobal takes the exclusive global lock, blocking every per-path
// operation. Used by bash, which may read or write any file in the
// workspace. The returned func releases it.
func (m *lockManager) acquireGlobal() func() {
	m.global.Lock()
	return m.global.Unlock
}
