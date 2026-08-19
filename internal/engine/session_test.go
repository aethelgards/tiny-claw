package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	context2 "github.com/aethelgards/tiny-claw/internal/context"
	"github.com/aethelgards/tiny-claw/internal/helper"
	"github.com/aethelgards/tiny-claw/internal/schema"
)

// ---- 测试辅助 ----

// contentMsg 构造 Content 为 n 个 'a' 的消息，estTokens = 4 + n/2。
func contentMsg(role schema.Role, n int) schema.Message {
	return schema.Message{Role: role, Content: strings.Repeat("a", n)}
}

func toolResultMsg(id string, n int) schema.Message {
	return schema.Message{Role: schema.RoleUser, Content: strings.Repeat("a", n), ToolCallID: id}
}

func toolCallMsg(name string, argsN int) schema.Message {
	return schema.Message{
		Role:    schema.RoleAssistant,
		Content: "ok",
		ToolCalls: []schema.ToolCall{{
			ID:        "tc-" + name,
			Name:      name,
			Arguments: []byte(strings.Repeat("a", argsN)),
		}},
	}
}

// fakeSummarizer 记录入参并返回固定结果。
func fakeSummarizer(t *testing.T, gotOld *string, gotDropped *[]schema.Message, result string, err error) context2.Summarizer {
	t.Helper()
	return func(_ context.Context, old string, msgs []schema.Message) (string, error) {
		*gotOld = old
		*gotDropped = msgs
		return result, err
	}
}

func mustLoadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}

func assertNoConsecutiveUsers(t *testing.T, msgs []schema.Message) {
	t.Helper()
	for i := 0; i < len(msgs)-1; i++ {
		if msgs[i].Role == schema.RoleUser && msgs[i+1].Role == schema.RoleUser {
			t.Fatalf("consecutive User messages at %d: %+v -> %+v", i, msgs[i], msgs[i+1])
		}
	}
}

// ---- 阈值计算 ----

func TestThresholdComputation(t *testing.T) {
	s := context2.NewSession("t1", t.TempDir(), context2.WithContextWindow(1000), context2.WithCompressRatio(80))
	if got := s.Threshold(); got != 800 {
		t.Fatalf("threshold = %d, want 800", got)
	}
	// ratio 越界钳制：<1 → 1
	s2 := context2.NewSession("t2", t.TempDir(), context2.WithContextWindow(1000), context2.WithCompressRatio(0))
	if got := s2.Threshold(); got != 10 {
		t.Fatalf("threshold(clamp low) = %d, want 10", got)
	}
	// ratio 越界钳制：>100 → 100
	s3 := context2.NewSession("t3", t.TempDir(), context2.WithContextWindow(1000), context2.WithCompressRatio(150))
	if got := s3.Threshold(); got != 1000 {
		t.Fatalf("threshold(clamp high) = %d, want 1000", got)
	}
	// 默认值：128k × 80%
	s4 := context2.NewSession("t4", t.TempDir())
	if got := s4.Threshold(); got != 128_000*80/100 {
		t.Fatalf("threshold(default) = %d, want %d", got, 128_000*80/100)
	}
}

func TestLookupContextWindow(t *testing.T) {
	cases := []struct {
		model string
		want  int
	}{
		{"claude-sonnet-4-5", 200_000},
		{"glm-4.6", 128_000},
		{"gpt-4o", 128_000},
		{"gpt-4.1", 128_000},
		{"deepseek-chat", 128_000},
		{"qwen-max", 128_000},
		{"unknown-model-x", 128_000},
	}
	for _, c := range cases {
		if got := context2.LookupContextWindow(c.model); got != c.want {
			t.Errorf("LookupContextWindow(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}

func TestWindowPriority(t *testing.T) {
	// 显式 WithContextWindow 优先于 WithModel，与选项顺序无关
	s := context2.NewSession("p1", t.TempDir(), context2.WithModel("claude-sonnet-4-5"), context2.WithContextWindow(5000))
	if got := s.Threshold(); got != 5000*80/100 {
		t.Fatalf("explicit window overridden by model: threshold=%d, want %d", got, 5000*80/100)
	}
	s2 := context2.NewSession("p2", t.TempDir(), context2.WithContextWindow(5000), context2.WithModel("claude-sonnet-4-5"))
	if got := s2.Threshold(); got != 5000*80/100 {
		t.Fatalf("explicit window overridden by model (reversed): threshold=%d, want %d", got, 5000*80/100)
	}
	// 仅 WithModel：查表
	s3 := context2.NewSession("p3", t.TempDir(), context2.WithModel("claude-sonnet-4-5"))
	if got := s3.Threshold(); got != 200_000*80/100 {
		t.Fatalf("model lookup window wrong: threshold=%d, want %d", got, 200_000*80/100)
	}
}

// ---- 触发行为 ----

// 低于阈值不压缩。
func TestNoCompressUnderThreshold(t *testing.T) {
	var old string
	var dropped []schema.Message
	s := context2.NewSession("nc", t.TempDir(), context2.WithContextWindow(2000), // threshold=1600
		context2.WithSummarizer(fakeSummarizer(t, &old, &dropped, "s", nil)))

	ctx := context.Background()
	for i := 0; i < 7; i++ { // 7 × 204 = 1428 ≤ 1600
		s.Append(ctx, contentMsg(schema.RoleUser, 400))
	}
	if old != "" || dropped != nil || s.Summary() != "" {
		t.Fatalf("compression triggered under threshold: old=%q dropped=%v", old, dropped)
	}
	if len(s.History()) != 7 {
		t.Fatalf("history len = %d, want 7", len(s.History()))
	}
}

// 超阈值触发压缩：摘要 + 保留最近原始消息，总估算 ≤ 预算。
func TestCompressTriggersOnAppend(t *testing.T) {
	var old string
	var dropped []schema.Message
	s := context2.NewSession("ct", t.TempDir(), context2.WithContextWindow(2000), // threshold=1600, budget=1400
		context2.WithSummarizer(fakeSummarizer(t, &old, &dropped, "s1", nil)))

	ctx := context.Background()
	for i := 0; i < 7; i++ {
		s.Append(ctx, contentMsg(schema.RoleUser, 400))
	}
	s.Append(ctx, contentMsg(schema.RoleUser, 400)) // 8 × 204 = 1632 > 1600 → 压缩

	if len(dropped) != 2 {
		t.Fatalf("dropped msgs = %d, want 2", len(dropped))
	}
	if s.Summary() != "s1" {
		t.Fatalf("summary = %q, want s1", s.Summary())
	}
	if len(s.History()) != 6 {
		t.Fatalf("history len = %d, want 6", len(s.History()))
	}
	if got := s.TotalTokens(); got > 1400 {
		t.Fatalf("total tokens after compress = %d, exceeds budget 1400", got)
	}
}

// 不同 ratio 在不同触发点触发。
func TestCompressRatioTriggerPoints(t *testing.T) {
	ctx := context.Background()
	msg := contentMsg(schema.RoleUser, 400) // 204 tokens

	triggerAt := func(ratio int) int {
		var old string
		var dropped []schema.Message
		s := context2.NewSession(t.Name(), t.TempDir(), context2.WithContextWindow(2000), context2.WithCompressRatio(ratio),
			context2.WithSummarizer(fakeSummarizer(t, &old, &dropped, "s", nil)))
		for i := 1; ; i++ {
			s.Append(ctx, msg)
			if s.Summary() != "" {
				return i
			}
		}
	}

	if got := triggerAt(50); got != 5 { // 5×204=1020 > 1000
		t.Fatalf("ratio 50 triggered at %d msgs, want 5", got)
	}
	if got := triggerAt(100); got != 10 { // 10×204=2040 > 2000
		t.Fatalf("ratio 100 triggered at %d msgs, want 10", got)
	}
}

// ---- 边界完整性 ----

// 截断边界不落在工具对中间：assistant(ToolCalls) + 结果 整体保留或整体丢弃。
func TestToolPairBoundaryIntact(t *testing.T) {
	// 工具对被整体丢弃的场景：budget 落在结果处 → 修正后从下一 assistant 开始
	var dropped []schema.Message
	s := context2.NewSession("tp", t.TempDir(), context2.WithContextWindow(2000), context2.WithCompressRatio(30), // threshold=600, budget=525
		context2.WithSummarizer(fakeSummarizer(t, new(string), &dropped, "s1", nil)))

	ctx := context.Background()
	m0 := contentMsg(schema.RoleUser, 700)     // est 354
	m1 := toolCallMsg("bash", 600)             // est 305
	m2 := toolResultMsg("tc-bash", 600)        // est 304
	m3 := contentMsg(schema.RoleAssistant, 50) // est 29
	s.Append(ctx, m0, m1, m2, m3)              // total 992 > 600

	// cutoff 自然落在 m2(工具结果) → 边界修正前移到 m3(assistant)
	if len(s.History()) != 1 || s.History()[0].Role != schema.RoleAssistant {
		t.Fatalf("history after compress = %+v, want [assistant]", s.History())
	}
	if len(dropped) != 3 {
		t.Fatalf("dropped = %d msgs, want 3 (pair must drop together)", len(dropped))
	}
}

// 压缩后输出序列无连续 User（摘要前置 + 交替性）。
func TestAlternationAndSummaryPlacement(t *testing.T) {
	var dropped []schema.Message
	s := context2.NewSession("alt", t.TempDir(), context2.WithContextWindow(1000), context2.WithCompressRatio(80), // threshold=800, budget=700
		context2.WithSummarizer(fakeSummarizer(t, new(string), &dropped, "sum", nil)))

	ctx := context.Background()
	s.Append(ctx,
		contentMsg(schema.RoleUser, 800),      // est 404
		contentMsg(schema.RoleAssistant, 800), // est 404
		contentMsg(schema.RoleUser, 50),       // est 29
		contentMsg(schema.RoleAssistant, 50),  // est 29
		contentMsg(schema.RoleUser, 50),       // est 29
		contentMsg(schema.RoleAssistant, 50),  // est 29
	) // total 924 > 800

	out := s.GetWorkingMemory(ctx)
	if len(out) != 6 { // 摘要 + 5 条保留
		t.Fatalf("working memory len = %d, want 6", len(out))
	}
	if out[0].Role != schema.RoleUser || !strings.HasPrefix(out[0].Content, "【历史会话摘要】") {
		t.Fatalf("first message not summary: %+v", out[0])
	}
	assertNoConsecutiveUsers(t, out)

	// 返回副本：修改返回值不影响会话内部状态
	out[0].Content = "mutated"
	out2 := s.GetWorkingMemory(ctx)
	if strings.HasPrefix(out2[0].Content, "mutated") {
		t.Fatal("GetWorkingMemory returned shared state")
	}
}

// history 以 User 开头（压缩发生在用户追问后）时摘要前置合并进第一条 User。
func TestSummaryMergedIntoLeadingUser(t *testing.T) {
	s := context2.NewSession("mrg", t.TempDir(), context2.WithContextWindow(1000), // threshold=800, budget=700
		context2.WithSummarizer(fakeSummarizer(t, new(string), new([]schema.Message), "sum", nil)))

	ctx := context.Background()
	// [assistant(800) → user(800)]：total 808 > 800 触发压缩，后缀仅剩最后的用户追问
	s.Append(ctx,
		contentMsg(schema.RoleAssistant, 800), // est 404
		contentMsg(schema.RoleUser, 800),      // est 404
	)

	out := s.GetWorkingMemory(ctx)
	if len(out) != 1 {
		t.Fatalf("working memory len = %d, want 1", len(out))
	}
	if out[0].Role != schema.RoleUser || !strings.HasPrefix(out[0].Content, "【历史会话摘要】") {
		t.Fatalf("summary not merged into leading user: %+v", out[0])
	}
	if !strings.Contains(out[0].Content, strings.Repeat("a", 800)) {
		t.Fatal("merged message lost original user content")
	}
	assertNoConsecutiveUsers(t, out)
}

// ---- 回退路径 ----

// summarizer 返回错误 → 回退纯截断（summary 置空、历史仍被裁剪）。
func TestSummarizerErrorFallback(t *testing.T) {
	s := context2.NewSession("ef", t.TempDir(), context2.WithContextWindow(2000),
		context2.WithSummarizer(fakeSummarizer(t, new(string), new([]schema.Message), "", errors.New("boom"))))

	ctx := context.Background()
	for i := 0; i < 8; i++ {
		s.Append(ctx, contentMsg(schema.RoleUser, 400))
	}
	if s.Summary() != "" {
		t.Fatalf("summary = %q, want empty on error", s.Summary())
	}
	if len(s.History()) != 6 {
		t.Fatalf("history len = %d, want 6 (truncation still applied)", len(s.History()))
	}
}

// 未注入 summarizer → 纯截断，不 panic。
func TestNoSummarizerTruncation(t *testing.T) {
	s := context2.NewSession("ns", t.TempDir(), context2.WithContextWindow(2000))

	ctx := context.Background()
	for i := 0; i < 8; i++ {
		s.Append(ctx, contentMsg(schema.RoleUser, 400))
	}
	if s.Summary() != "" {
		t.Fatalf("summary = %q, want empty", s.Summary())
	}
	if len(s.History()) != 6 {
		t.Fatalf("history len = %d, want 6", len(s.History()))
	}
}

// 单回合超预算（最后一条消息放不进预算）：不压缩、不 panic、summarizer 不被调用。
func TestSingleTurnOverBudgetAbort(t *testing.T) {
	called := false
	s := context2.NewSession("sb", t.TempDir(), context2.WithContextWindow(2000), context2.WithCompressRatio(20), // threshold=400, budget=350
		context2.WithSummarizer(func(_ context.Context, _ string, _ []schema.Message) (string, error) {
			called = true
			return "s", nil
		}))

	ctx := context.Background()
	s.Append(ctx, contentMsg(schema.RoleUser, 1000)) // est 504 > budget 350
	if called {
		t.Fatal("summarizer called on single-turn over-budget")
	}
	if len(s.History()) != 1 || s.Summary() != "" {
		t.Fatalf("history=%+v summary=%q, want unchanged", s.History(), s.Summary())
	}
}

// 边界修正会清空后缀（当前回合工具结果未回复）：放弃压缩，不产生孤儿工具结果。
func TestBoundaryFixWouldDropCurrentTurnAbort(t *testing.T) {
	called := false
	s := context2.NewSession("bf", t.TempDir(), context2.WithContextWindow(2000), context2.WithCompressRatio(30), // threshold=600, budget=525
		context2.WithSummarizer(func(_ context.Context, _ string, _ []schema.Message) (string, error) {
			called = true
			return "s", nil
		}))

	ctx := context.Background()
	s.Append(ctx,
		toolCallMsg("bash", 600),      // est 305
		toolResultMsg("tc-bash", 600), // est 304
	) // total 609 > 600

	if called {
		t.Fatal("summarizer called when boundary fix would drop current turn")
	}
	if len(s.History()) != 2 || s.Summary() != "" {
		t.Fatalf("history=%+v summary=%q, want unchanged", s.History(), s.Summary())
	}
}

// 摘要存在但 history 为空：GetWorkingMemory 仅返回摘要消息。
func TestWorkingMemorySummaryOnly(t *testing.T) {
	s := context2.NewSession("so", t.TempDir())
	s.SetSummary("direct")
	out := s.GetWorkingMemory(context.Background())
	if len(out) != 1 || out[0].Role != schema.RoleUser || !strings.HasPrefix(out[0].Content, "【历史会话摘要】") {
		t.Fatalf("summary-only working memory wrong: %+v", out)
	}
}

// ---- 持久化 ----

// Append 追加写盘 → 压缩原子重写 → LoadSession round-trip 恢复摘要 + 历史。
func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	var dropped []schema.Message
	s := context2.NewSession("sess-1", dir, context2.WithContextWindow(2000),
		context2.WithSummarizer(fakeSummarizer(t, new(string), &dropped, "s1", nil)))

	ctx := context.Background()
	for i := 0; i < 7; i++ {
		s.Append(ctx, contentMsg(schema.RoleUser, 400))
	}
	file := filepath.Join(dir, "sessions", "sess-1.json")
	raw := mustLoadFile(t, file)
	if lines := strings.Count(strings.TrimSpace(raw), "\n") + 1; lines != 7 {
		t.Fatalf("file lines before compress = %d, want 7", lines)
	}
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		var probe struct {
			Role string `json:"role"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err != nil || probe.Role == "" {
			t.Fatalf("line before compress is not a message: %s", line)
		}
	}

	s.Append(ctx, contentMsg(schema.RoleUser, 400)) // 触发压缩 → 重写文件

	raw = mustLoadFile(t, file)
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 7 { // 1 摘要 + 6 消息
		t.Fatalf("file lines after compress = %d, want 7", len(lines))
	}
	var first struct {
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil || first.Summary != "s1" {
		t.Fatalf("first line after compress not summary record: %s", lines[0])
	}

	loaded, err := context2.LoadSession("sess-1", dir, context2.WithContextWindow(2000))
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded.Summary() != "s1" {
		t.Fatalf("loaded summary = %q, want s1", loaded.Summary())
	}
	if len(loaded.History()) != 6 {
		t.Fatalf("loaded history len = %d, want 6", len(loaded.History()))
	}

	// 缺失文件 → 空会话，不报错
	ghost, err := context2.LoadSession("ghost", dir)
	if err != nil || len(ghost.History()) != 0 || ghost.Summary() != "" {
		t.Fatalf("LoadSession(missing) = %+v, %v; want empty session", ghost, err)
	}
}

// 文件含非 JSON 行：跳过、不报错，摘要与消息仍恢复。
func TestLoadSessionSkipsInvalidLine(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "sessions", "dirty.json")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "not json at all\n" +
		helper.Any2Json(map[string]string{"summary": "s9"}) + "\n" +
		helper.Any2Json(schema.Message{Role: schema.RoleUser, Content: "hi"}) + "\n" +
		helper.Any2Json(schema.Message{Role: schema.RoleSystem, Content: "sys"}) + "\n"
	if err := os.WriteFile(file, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := context2.LoadSession("dirty", dir)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded.Summary() != "s9" || len(loaded.History()) != 1 || loaded.History()[0].Content != "hi" {
		t.Fatalf("loaded summary=%q history=%+v, want s9 + [hi] (system line skipped)", loaded.Summary(), loaded.History())
	}
}

// 会话路径是目录 → ReadFile 报非"不存在"错误 → LoadSession 返回错误。
func TestLoadSessionReadError(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "sessions", "sess.json")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := context2.LoadSession("sess", dir); err == nil {
		t.Fatal("LoadSession should fail when session path is a directory")
	}
}

// 磁盘写入失败（sessions 路径被普通文件占据）：仅告警，内存态压缩仍生效，不 panic。
func TestDiskErrorsNoPanic(t *testing.T) {
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var dropped []schema.Message
	s := context2.NewSession("disk", blocked, context2.WithContextWindow(2000),
		context2.WithSummarizer(fakeSummarizer(t, new(string), &dropped, "s1", nil)))

	ctx := context.Background()
	for i := 0; i < 8; i++ {
		s.Append(ctx, contentMsg(schema.RoleUser, 400))
	}
	if s.Summary() != "s1" {
		t.Fatalf("summary = %q, want s1 despite disk errors", s.Summary())
	}
	if len(s.History()) != 6 {
		t.Fatalf("history len = %d, want 6 (in-memory compression still applied)", len(s.History()))
	}
	if !s.Mu.TryLock() {
		t.Fatal("session mutex left locked")
	}
	s.Mu.Unlock()
}

// system 提示词误入 Append → 丢弃 + 告警：不进 history、不落盘。
func TestAppendDropsSystemMessage(t *testing.T) {
	dir := t.TempDir()
	s := context2.NewSession("sys", dir, context2.WithContextWindow(2000))

	ctx := context.Background()
	s.Append(ctx,
		contentMsg(schema.RoleUser, 10),
		contentMsg(schema.RoleSystem, 500), // 应被丢弃
		contentMsg(schema.RoleAssistant, 10),
	)

	if len(s.History()) != 2 {
		t.Fatalf("history len = %d, want 2 (system dropped)", len(s.History()))
	}
	for _, m := range s.History() {
		if m.Role == schema.RoleSystem {
			t.Fatalf("system message leaked into history: %+v", s.History())
		}
	}

	raw := mustLoadFile(t, filepath.Join(dir, "sessions", "sys.json"))
	if strings.Contains(raw, "System") {
		t.Fatalf("system message persisted: %s", raw)
	}
	if lines := strings.Count(strings.TrimSpace(raw), "\n") + 1; lines != 2 {
		t.Fatalf("file lines = %d, want 2", lines)
	}

	// 全部为 system → 直接返回，不建文件、不 panic
	s2 := context2.NewSession("sys2", dir, context2.WithContextWindow(2000))
	s2.Append(ctx, contentMsg(schema.RoleSystem, 500))
	if len(s2.History()) != 0 {
		t.Fatalf("all-system append left history len = %d, want 0", len(s2.History()))
	}
	if _, err := os.Stat(filepath.Join(dir, "sessions", "sys2.json")); !os.IsNotExist(err) {
		t.Fatal("all-system append should not create session file")
	}
}
