package trace

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type traceKey struct{}

type Span struct {
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	DurationMS int64
	Attributes map[string]any
	Children   []*Span
	mu         sync.Mutex
}

func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	span := &Span{
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]any),
	}

	if parent, ok := ctx.Value(traceKey{}).(*Span); ok {
		parent.mu.Lock()
		parent.Children = append(parent.Children, span)
		parent.mu.Unlock()
	}

	newCtx := context.WithValue(ctx, traceKey{}, span)

	return newCtx, span
}

func (s *Span) EndSpan() {
	if !s.EndTime.IsZero() {
		return
	}
	s.EndTime = time.Now()
	s.DurationMS = s.EndTime.Sub(s.StartTime).Milliseconds()
}

func (s *Span) AddAttribute(key string, value any) {
	s.mu.Lock()
	s.Attributes[key] = value
	s.mu.Unlock()
}

func ExportTraceToFile(rootSpan *Span, workDir string, sessionID string) error {
	traceDir := filepath.Join(workDir, ".claw", "traces")
	if err := os.MkdirAll(traceDir, os.ModePerm); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to create trace directory: %s", traceDir))
	}

	traceFilePath := filepath.Join(traceDir, fmt.Sprintf("trace_%s_%d.json", sessionID, time.Now().Unix()))

	data, err := json.MarshalIndent(rootSpan, "", "  ")
	if err != nil {
		return errors.WithStack(err)
	}
	return errors.Wrap(os.WriteFile(traceFilePath, data, os.ModePerm), "failed to write trace file")
}
