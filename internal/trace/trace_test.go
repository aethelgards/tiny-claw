package trace

import (
	"context"
	"testing"
)

func TestStartSpanAttachesChildToParent(t *testing.T) {
	ctx, root := StartSpan(context.Background(), "root")
	ctx, child := StartSpan(ctx, "child")
	_ = child
	root.EndSpan()

	if len(root.Children) != 1 {
		t.Fatalf("expected root to have 1 child, got %d", len(root.Children))
	}
	if root.Children[0].Name != "child" {
		t.Fatalf("expected child name 'child', got %q", root.Children[0].Name)
	}
}

func TestStartSpanAttachesGrandchild(t *testing.T) {
	ctx, root := StartSpan(context.Background(), "root")
	ctx, child := StartSpan(ctx, "child")
	_, grandchild := StartSpan(ctx, "grandchild")
	grandchild.EndSpan()
	child.EndSpan()
	root.EndSpan()

	if len(root.Children) != 1 {
		t.Fatalf("expected root to have 1 child, got %d", len(root.Children))
	}
	if len(root.Children[0].Children) != 1 {
		t.Fatalf("expected child to have 1 grandchild, got %d", len(root.Children[0].Children))
	}
}
