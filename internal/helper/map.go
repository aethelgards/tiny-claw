package helper

import "sync"

type SyncMap[K comparable, V any] struct {
	sm sync.Map
}

func (s *SyncMap[K, V]) Put(k K, v V) {
	s.sm.Store(k, v)
}

func (s *SyncMap[K, V]) ToMap() map[K]V {
	var resMap = make(map[K]V)
	s.sm.Range(func(k, v any) bool {
		resMap[k.(K)] = v.(V)
		return true
	})
	return resMap
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{}
}
