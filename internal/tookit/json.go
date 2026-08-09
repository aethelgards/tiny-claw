package tookit

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func Any2Json(any any) string {
	raw, _ := json.Marshal(any)
	return string(raw)
}

// AppendLine 以追加模式向文件写入一行（自动创建缺失目录）。
func AppendLine(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return err
	}
	return nil
}
