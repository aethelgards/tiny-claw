package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aethelgards/tiny-claw/internal/schema"
)

// defaultBashTimeout bounds how long a single bash invocation may run before
// the process is killed.
const defaultBashTimeout = 120 * time.Second

// maxOutputLen caps the number of bytes of command output returned to the
// model. Kept as a var so tests can shrink it without spawning chatty
// commands.
var maxOutputLen = 8192

type BashTool struct {
	workDir string
	timeout time.Duration
}

// BashOption configures a BashTool at construction time.
type BashOption func(*BashTool)

// WithTimeout overrides the default command timeout. A zero duration
// disables the timeout.
func WithTimeout(timeout time.Duration) BashOption {
	return func(t *BashTool) { t.timeout = timeout }
}

func (b *BashTool) Name() string {
	return "bash"
}

func (b *BashTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name:        b.Name(),
		Description: "在指定工作区目录下执行 shell 命令并返回其输出。命令的工作目录为工作区根目录；可通过可选参数 timeout（秒）覆盖默认超时。",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "要执行的 shell 命令，如 go build ./...",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "命令超时时间（秒），默认 120",
				},
			},
			"required": []string{
				"command",
			},
		},
	}
}

type bashArgs struct {
	Command string  `json:"command"`
	Timeout float64 `json:"timeout"`
}

func (b *BashTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var bashArgs bashArgs
	if err := json.Unmarshal(args, &bashArgs); err != nil {
		return "", fmt.Errorf("bash failed, input json convert to bashArgs struct failed, fail info: %s", err.Error())
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(bashArgs.Command) == "" {
		return "", errors.New("command cannot be empty")
	}

	// A shell command may read or write any file in the workspace, so it must
	// exclude concurrent readers and mutators for its whole run.
	release := toolLocks.acquireGlobal()
	defer release()

	timeout := b.timeout
	if bashArgs.Timeout > 0 {
		timeout = time.Duration(bashArgs.Timeout * float64(time.Second))
	}
	runCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	shell, shellArgs := shellCommand()
	argv := append(shellArgs, bashArgs.Command)
	cmd := exec.CommandContext(runCtx, shell, argv...)
	cmd.Dir = b.workDir
	out := newBoundedBuffer(maxOutputLen)
	cmd.Stdout = out
	cmd.Stderr = out

	err := cmd.Run()
	if err != nil {
		// A cancelled or timed-out context wins over the kill signal exec
		// reports back, so callers can distinguish timeout from a plain
		// non-zero exit.
		if runCtx.Err() != nil {
			return "", fmt.Errorf("command %q: %w", bashArgs.Command, runCtx.Err())
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// A non-zero exit is ordinary command failure data, not a tool
			// error; the output and exit code are returned to the model.
			return fmt.Sprintf("%s\n\n命令退出码: %d", out.String(), exitErr.ExitCode()), nil
		}
		return "", fmt.Errorf("run command failed, fail info: %s", err.Error())
	}
	return fmt.Sprintf("%s\n\n命令退出码: 0", out.String()), nil
}

// shellCommand returns the argv used to run user-supplied command lines:
// bash on unix-likes, cmd on windows.
func shellCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/C"}
	}
	return "bash", []string{"-c"}
}

// boundedBuffer keeps the first max bytes written to it and tracks whether
// more data arrived. It is safe for concurrent use by the stdout and stderr
// copier goroutines, and its memory use stays O(max) regardless of how much
// the command emits.
type boundedBuffer struct {
	mu        sync.Mutex
	max       int
	buf       []byte
	truncated bool
}

func newBoundedBuffer(max int) *boundedBuffer {
	return &boundedBuffer{max: max}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buf) < b.max {
		room := b.max - len(b.buf)
		if len(p) > room {
			b.buf = append(b.buf, p[:room]...)
			b.truncated = true
		} else {
			b.buf = append(b.buf, p...)
		}
	} else {
		b.truncated = true
	}
	// Always report the full length: the pipe copier must not see a short
	// write, or exec would treat it as an I/O error.
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := string(b.buf)
	if b.truncated {
		s += fmt.Sprintf("\n\n...[输出过长，已截断：仅展示前 %d 字节]...\n", b.max)
	}
	return s
}

func NewBashTool(workDir string, opts ...BashOption) BaseTool {
	t := &BashTool{
		workDir: workDir,
		timeout: defaultBashTimeout,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}
