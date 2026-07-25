package detector

import (
	"context"
	"testing"
	"time"
)

func TestLocalBatchThreshold(t *testing.T) {
	d := NewLocal(BatchConfig{Window: time.Second, Threshold: 3, MaxFactor: 5})
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		res, err := d.Record(ctx, "actor-1")
		if err != nil {
			t.Fatal(err)
		}
		if res.Abusive {
			t.Fatalf("call %d should not be abusive yet", i+1)
		}
	}

	res, err := d.Record(ctx, "actor-1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Abusive {
		t.Fatal("4th call in 1s window should be abusive")
	}
	if res.CostFactor <= 1 {
		t.Fatalf("expected cost factor > 1, got %v", res.CostFactor)
	}
}

func TestLocalBatchWindowExpiry(t *testing.T) {
	d := NewLocal(BatchConfig{Window: 100 * time.Millisecond, Threshold: 2, MaxFactor: 4})
	ctx := context.Background()

	_, _ = d.Record(ctx, "k")
	_, _ = d.Record(ctx, "k")
	res, _ := d.Record(ctx, "k")
	if !res.Abusive {
		t.Fatal("expected abusive before window expiry")
	}

	time.Sleep(120 * time.Millisecond)

	res, err := d.Record(ctx, "k")
	if err != nil {
		t.Fatal(err)
	}
	if res.Abusive {
		t.Fatal("events outside window should reset count")
	}
}

func TestCostFactorCap(t *testing.T) {
	cfg := BatchConfig{Window: time.Minute, Threshold: 1, MaxFactor: 3}
	res := cfg.costFactor(100)
	if res.CostFactor > 3 {
		t.Fatalf("cost factor should cap at MaxFactor, got %v", res.CostFactor)
	}
}
