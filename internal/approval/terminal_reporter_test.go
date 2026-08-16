package approval

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aethelgards/tiny-claw/internal/reporter"
)

// neverRead 永不返回的 reader：模拟用户不输入（超时路径）。
type neverRead struct{}

func (neverRead) Read(p []byte) (int, error) {
	<-make(chan struct{})
	return 0, io.EOF
}

// runWaitingForApproval 在 goroutine 里跑 WaitingForApproval（终端 reporter 在钩子内自行 ResolveApproval），
// 带 2s 兜底超时，返回结论。
func runWaitingForApproval(t *testing.T, mgr *ApprovalManager, rep reporter.Reporter) (bool, string) {
	t.Helper()
	got := make(chan struct {
		allowed bool
		reason  string
	}, 1)
	go func() {
		allowed, reason := mgr.WaitingForApproval(context.Background(), "task-1", "bash", "rm -rf /", rep, "local")
		got <- struct {
			allowed bool
			reason  string
		}{allowed, reason}
	}()
	select {
	case r := <-got:
		return r.allowed, r.reason
	case <-time.After(2 * time.Second):
		t.Fatal("WaitingForApproval 未在兜底超时内返回")
		return false, ""
	}
}

func TestTerminalReporterApprove(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("y\n"), true)
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if !allowed || reason != "" {
		t.Fatalf("y 应放行，得到 (%v, %q)", allowed, reason)
	}
}

func TestTerminalReporterReject(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("n\n"), true)
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || reason != "终端用户拒绝" {
		t.Fatalf("n 应拒绝，得到 (%v, %q)", allowed, reason)
	}
}

func TestTerminalReporterInputVariants(t *testing.T) {
	cases := []struct {
		input   string
		allowed bool
		reason  string
	}{
		{"Y\n", true, ""},
		{"yes\n", true, ""},
		{"YES\n", true, ""},
		{"Yes\n", true, ""},
		{"N\n", false, "终端用户拒绝"},
		{"no\n", false, "终端用户拒绝"},
		{"whatever\n", false, "终端用户拒绝"},
		{"\n", false, "终端用户拒绝"},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			mgr := NewApprovalManager(time.Minute)
			rep := newTerminalReporterForTest(mgr, strings.NewReader(c.input), true)
			allowed, reason := runWaitingForApproval(t, mgr, rep)
			if allowed != c.allowed || reason != c.reason {
				t.Fatalf("输入 %q 期望 (%v, %q)，得到 (%v, %q)", c.input, c.allowed, c.reason, allowed, reason)
			}
		})
	}
}

func TestTerminalReporterEOF(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("y"), true) // 无换行 → EOF
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || reason != "终端输入读取失败" {
		t.Fatalf("EOF 应 fail-closed，得到 (%v, %q)", allowed, reason)
	}
}

func TestTerminalReporterNonInteractive(t *testing.T) {
	mgr := NewApprovalManager(time.Minute)
	rep := newTerminalReporterForTest(mgr, strings.NewReader("y\n"), false)
	start := time.Now()
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || reason != "非交互终端，自动拒绝" {
		t.Fatalf("非交互应拒绝，得到 (%v, %q)", allowed, reason)
	}
	if time.Since(start) > time.Second {
		t.Fatal("非交互拒绝不应阻塞")
	}
}

func TestTerminalReporterTimeout(t *testing.T) {
	mgr := NewApprovalManager(50 * time.Millisecond)
	rep := newTerminalReporterForTest(mgr, neverRead{}, true)
	allowed, reason := runWaitingForApproval(t, mgr, rep)
	if allowed || !strings.Contains(reason, "审批超时") {
		t.Fatalf("不输入应超时拒绝，得到 (%v, %q)", allowed, reason)
	}
}
