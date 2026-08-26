package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// MemoryType 记忆类型，也是 JSONL 文件名（如 preferences.jsonl）
type MemoryType string

const (
	TypePreference MemoryType = "preferences" // 用户偏好（跨项目适用）
	TypeProject    MemoryType = "project"     // 项目知识
	TypeError      MemoryType = "errors"      // 错误模式
	TypeTool       MemoryType = "tools"       // 工具策略
)

var AllTypes = []MemoryType{TypePreference, TypeProject, TypeError, TypeTool}

// ValidType 检查字符串是否为合法类型，返回规范化类型
func ValidType(s string) (MemoryType, bool) {
	for _, t := range AllTypes {
		if string(t) == s {
			return t, true
		}
	}
	return "", false
}

// Scope 记忆作用域
type Scope string

const (
	ScopeGlobal  Scope = "global"  // ~/.claw/memory/
	ScopeProject Scope = "project" // .claw/memory/
)

// Memory 单条记忆
type Memory struct {
	ID           string     `json:"id"`                     // = MemoryID(type, content)，16 字符短哈希
	Type         MemoryType `json:"type"`                   // 类型（决定文件归属与 scope 路由）
	Content      string     `json:"content"`                // 记忆正文（一句话/一段话）
	Embedding    []float32  `json:"embedding,omitempty"`    // 语义向量（可选，仅 v2 格式）
	Source       string     `json:"source,omitempty"`       // 来源：sessionID 或 "explicit"
	SessionID    string     `json:"sessionId,omitempty"`    // 产生该记忆的会话
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	AccessCount  int64      `json:"accessCount"`            // 被显式 recall 引用次数（衰减依据）
	LastAccessAt time.Time  `json:"lastAccessAt,omitempty"` // 最近引用时间
}

// MemoryID 由类型 + 归一化正文计算确定性 ID（R6）：
// 相同内容重复 Save 天然幂等（upsert 刷新 UpdatedAt），杜绝近重复堆积。
func MemoryID(t MemoryType, content string) string {
	norm := strings.Join(strings.Fields(content), " ") // 折叠空白
	sum := sha256.Sum256([]byte(string(t) + "\x00" + norm))
	return hex.EncodeToString(sum[:8])
}

// scopeOfType 按类型路由存储位置：用户偏好全局复用（P4），其余归属项目。
func scopeOfType(t MemoryType) Scope {
	if t == TypePreference {
		return ScopeGlobal
	}
	return ScopeProject
}

// versionLine 文件版本号（首行版本头行）
const versionLine = 2