package db

import (
	"context"
	"testing"
)

func TestPing_NilPool(t *testing.T) {
	err := Ping(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil pool, got nil")
	}
	if err.Error() != "pool is nil" {
		t.Errorf("expected 'pool is nil' error, got '%s'", err.Error())
	}
}

func TestCheck_NilPool(t *testing.T) {
	status := Check(context.Background(), nil)

	if status.Healthy {
		t.Error("expected unhealthy status for nil pool")
	}
	if status.Error == nil {
		t.Error("expected error in status for nil pool")
	}
}

func TestWaitForReady_NilPool(t *testing.T) {
	err := WaitForReady(context.Background(), nil, 100)
	if err == nil {
		t.Error("expected error for nil pool, got nil")
	}
}
