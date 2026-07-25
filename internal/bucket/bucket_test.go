package bucket

import (
	"context"
	"testing"
	"time"
)

func TestLocalBucketRefillAndBurst(t *testing.T) {
	b := NewLocal(1.0)
	p := Params{Capacity: 5, Rate: 10.0 / 60.0, Weight: 1}

	// Consume burst capacity.
	for i := 0; i < 5; i++ {
		res, err := b.Consume(context.Background(), "k1", p, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	res, err := b.Consume(context.Background(), "k1", p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed {
		t.Fatal("6th immediate request should be denied")
	}
	if res.RetryAfter <= 0 {
		t.Fatal("expected positive retry-after")
	}

	// After refill window, one token should be available.
	wait := time.Duration(float64(time.Second) * 60.0 / 10.0)
	time.Sleep(wait + 50*time.Millisecond)

	res, err = b.Consume(context.Background(), "k1", p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed {
		t.Fatal("expected token after refill")
	}
}

func TestLocalBucketWeightPriority(t *testing.T) {
	b := NewLocal(1.0)
	normal := Params{Capacity: 1, Rate: 1.0, Weight: 1}
	priority := Params{Capacity: 1, Rate: 1.0, Weight: 3}

	_, _ = b.Consume(context.Background(), "normal", normal, 1)
	_, _ = b.Consume(context.Background(), "priority", priority, 1)

	time.Sleep(400 * time.Millisecond)

	rn, _ := b.Consume(context.Background(), "normal", normal, 1)
	rp, _ := b.Consume(context.Background(), "priority", priority, 1)

	if !rp.Allowed {
		t.Fatal("weighted bucket should refill faster")
	}
	if rn.Allowed {
		t.Fatal("normal bucket should still be empty after partial refill")
	}
}

func TestLocalFallbackScale(t *testing.T) {
	full := NewLocal(1.0)
	half := NewLocal(0.5)
	p := Params{Capacity: 10, Rate: 10.0 / 60.0, Weight: 1}

	for i := 0; i < 5; i++ {
		r, _ := half.Consume(context.Background(), "k", p, 1)
		if !r.Allowed {
			t.Fatalf("half scale burst %d denied unexpectedly", i+1)
		}
	}
	r, _ := half.Consume(context.Background(), "k", p, 1)
	if r.Allowed {
		t.Fatal("half scale should cap burst at 5")
	}

	r, _ = full.Consume(context.Background(), "k2", p, 1)
	if !r.Allowed {
		t.Fatal("full scale first request should pass")
	}
}

func TestHardBlockZeroRate(t *testing.T) {
	b := NewLocal(1.0)
	p := Params{Capacity: 0, Rate: 0, Weight: 1}

	res, err := b.Consume(context.Background(), "blocked", p, 1)
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed {
		t.Fatal("zero capacity should always deny")
	}
	if res.RetryAfter != time.Minute {
		t.Fatalf("expected 1m retry, got %v", res.RetryAfter)
	}
}
