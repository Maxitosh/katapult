package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestIsReregistrationNeeded_FailedPrecondition(t *testing.T) {
	err := status.Error(codes.FailedPrecondition, "agent is disconnected: must re-register")
	if !isReregistrationNeeded(err) {
		t.Fatal("expected FailedPrecondition to trigger re-registration")
	}
}

func TestIsReregistrationNeeded_MustReRegisterString(t *testing.T) {
	err := fmt.Errorf("heartbeat failed: agent abc is disconnected: must re-register")
	if !isReregistrationNeeded(err) {
		t.Fatal("expected 'must re-register' string to trigger re-registration")
	}
}

func TestIsReregistrationNeeded_OtherError(t *testing.T) {
	err := fmt.Errorf("heartbeat failed: connection refused")
	if isReregistrationNeeded(err) {
		t.Fatal("expected generic error NOT to trigger re-registration")
	}
}

func TestRetryReregister_Success(t *testing.T) {
	c := &Client{logger: noopLogger()}
	attempt := 0
	regFunc := func() error {
		attempt++
		if attempt < 3 {
			return fmt.Errorf("transient failure")
		}
		return nil
	}

	err := c.retryReregister(context.Background(), regFunc, 5, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if attempt != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempt)
	}
}

func TestRetryReregister_AllFail(t *testing.T) {
	c := &Client{logger: noopLogger()}
	regFunc := func() error {
		return fmt.Errorf("permanent failure")
	}

	err := c.retryReregister(context.Background(), regFunc, 3, 1*time.Millisecond)
	if err == nil {
		t.Fatal("expected error after exhausted retries")
	}
}

func TestRetryReregister_ContextCancelled(t *testing.T) {
	c := &Client{logger: noopLogger()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	regFunc := func() error {
		return fmt.Errorf("failure")
	}

	err := c.retryReregister(ctx, regFunc, 10, 1*time.Second)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}
