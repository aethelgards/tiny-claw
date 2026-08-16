package approval

import (
	"context"
	"testing"
)

func TestApprovalContextRoundtrip(t *testing.T) {
	rep := &fakeApprovalReporter{}
	ctx := WithApprovalContext(context.Background(), rep, "ou_x")
	ac, ok := approvalContextFrom(ctx)
	if !ok {
		t.Fatal("approvalContextFrom 应返回 ok=true")
	}
	if ac.approverID != "ou_x" {
		t.Fatalf("approverID = %q, want ou_x", ac.approverID)
	}
	if ac.reporter != rep {
		t.Fatal("reporter 应原样回传")
	}
}

func TestApprovalContextMissing(t *testing.T) {
	if _, ok := approvalContextFrom(context.Background()); ok {
		t.Fatal("裸 ctx 应返回 ok=false")
	}
}
