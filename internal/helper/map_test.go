package helper

import "testing"

func TestSyncMap(t *testing.T) {
	syncMap := NewSyncMap[string, string]()
	syncMap.Put("key", "value")
	syncMap.Put("key2", "value")
	syncMap.Put("key3", "value")

	toMap := syncMap.ToMap()
	print(Any2Json(toMap))
}
