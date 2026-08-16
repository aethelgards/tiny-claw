package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/aethelgards/tiny-claw/internal/schema"
	"github.com/samber/lo"
)

type ReminderInjector struct {
	consecutiveFailures map[string]int
	maxFailedCount      int
}

var nudgeMsg = `
[SYSTEM REMINDER 警告] 你似乎陷入了死循环，你刚刚连续 %s，并且都失败了。
请立即停止这种无效的重试！！！你的注意力被当前报错过度吸引了。
你需要：
1. 停止猜测参数，跳出当前局部思维
2. 彻底改变你的策略
3. 如果你确实无法通过系统工具解决当前问题，请直接结束任务并向用户说明你需要什么帮助，而不是继续盲目猜测消耗API资源尝试。
`

func NewReminderInjector(maxFailedCount int) ReminderInjector {
	return ReminderInjector{
		consecutiveFailures: make(map[string]int),
		maxFailedCount:      maxFailedCount,
	}
}

func (r *ReminderInjector) checkOne(ctx context.Context, uk string, lastToolCall schema.ToolCall, lastResult schema.ToolResult) (string, bool) {
	uk = lastToolCall.Name
	if _, ok := r.consecutiveFailures[uk]; !ok {
		r.consecutiveFailures[uk] = 1
	} else {
		r.consecutiveFailures[uk]++
	}
	failCount := r.consecutiveFailures[uk]

	slog.WarnContext(ctx, "reminder check", slog.String("toolName", lastToolCall.Name), slog.Int("failCount", failCount))

	if failCount > r.maxFailedCount {
		slog.WarnContext(ctx, "reminder check over max check count", slog.String("toolName", lastToolCall.Name))
		return fmt.Sprintf("%d 次使用 %s 工具; ", failCount, lastToolCall.Name), true
	}
	return "", false
}

func (r *ReminderInjector) CheckAndRemind(ctx context.Context, callMap map[string]lo.Tuple2[schema.ToolCall, schema.ToolResult]) *schema.Message {
	if len(callMap) == 0 {
		r.consecutiveFailures = make(map[string]int)
		return nil
	}
	var sb strings.Builder
	var needRemind = false
	for uk, callInfo := range callMap {
		if msg, ok := r.checkOne(ctx, uk, callInfo.A, callInfo.B); ok {
			sb.WriteString(msg)
			needRemind = true
		}
	}

	if !needRemind {
		return nil
	}

	return &schema.Message{
		Role:    schema.RoleUser,
		Content: fmt.Sprintf(nudgeMsg, sb.String()),
	}
}
