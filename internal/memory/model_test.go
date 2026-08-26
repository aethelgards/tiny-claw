package memory

import (
	"testing"
)

func TestMemoryType_Valid(t *testing.T) {
	cases := []struct {
		input    string
		expected MemoryType
		ok       bool
	}{
		{"preferences", TypePreference, true},
		{"project", TypeProject, true},
		{"errors", TypeError, true},
		{"tools", TypeTool, true},
		{"invalid", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := ValidType(c.input)
		if ok != c.ok || got != c.expected {
			t.Errorf("ValidType(%q) = (%v, %v), want (%v, %v)", c.input, got, ok, c.expected, c.ok)
		}
	}
}

func TestMemoryID_Deterministic(t *testing.T) {
	id1 := MemoryID(TypeProject, "本项目用 Go 1.26 + gin")
	id2 := MemoryID(TypeProject, "本项目用 Go 1.26 + gin")
	if id1 != id2 {
		t.Errorf("MemoryID not deterministic: %s != %s", id1, id2)
	}
}

func TestMemoryID_DiffersByType(t *testing.T) {
	id1 := MemoryID(TypePreference, "相同内容")
	id2 := MemoryID(TypeProject, "相同内容")
	if id1 == id2 {
		t.Errorf("MemoryID should differ by type")
	}
}

func TestMemoryID_DiffersByContent(t *testing.T) {
	id1 := MemoryID(TypeProject, "内容A")
	id2 := MemoryID(TypeProject, "内容B")
	if id1 == id2 {
		t.Errorf("MemoryID should differ by content")
	}
}

func TestMemoryID_NormalizesWhitespace(t *testing.T) {
	id1 := MemoryID(TypeProject, "  多   空白  ")
	id2 := MemoryID(TypeProject, "多 空白")
	if id1 != id2 {
		t.Errorf("MemoryID should normalize whitespace: %s != %s", id1, id2)
	}
}

func TestScopeOfType(t *testing.T) {
	if scopeOfType(TypePreference) != ScopeGlobal {
		t.Errorf("preferences should be global")
	}
	for _, tpe := range []MemoryType{TypeProject, TypeError, TypeTool} {
		if scopeOfType(tpe) != ScopeProject {
			t.Errorf("%s should be project", tpe)
		}
	}
}

func TestMemory_Defaults(t *testing.T) {
	m := Memory{
		Type:    TypeProject,
		Content: "测试内容",
	}
	if !m.CreatedAt.IsZero() || !m.UpdatedAt.IsZero() {
		t.Errorf("raw Memory should have zero timestamps; Save() fills them")
	}
	if m.AccessCount != 0 {
		t.Errorf("AccessCount should default to 0")
	}
}