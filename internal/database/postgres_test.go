package database

import (
	"context"
	"testing"
)

func TestNewPoolRequiresURL(t *testing.T) {
	pool, err := NewPool(context.Background(), "  ")
	if err == nil {
		if pool != nil {
			pool.Close()
		}
		t.Fatal("expected empty database url to be rejected")
	}
}
