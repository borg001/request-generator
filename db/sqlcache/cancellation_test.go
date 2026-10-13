package sqlcache

import (
	"context"
	"database/sql/driver"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanceledLeaderDoesNotFailLiveWaiters(t *testing.T) {
	for _, cancelLeader := range []bool{false, true} {
		name := "ordinary_error"
		if cancelLeader {
			name = "canceled_leader"
		}
		t.Run(name, func(t *testing.T) {
			cache := testCache()
			leader, cancel := context.WithCancel(context.Background())
			defer cancel()
			started, release := make(chan struct{}), make(chan struct{})
			finished := make(chan error, 9)
			failure := errors.New("driver read interrupted")
			var calls atomic.Int32
			go func() {
				_, err := cache.query(leader, "SELECT value", nil, true, func() (driver.Rows, error) { calls.Add(1); close(started); <-release; return nil, failure })
				finished <- err
			}()
			<-started
			ctx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			for i := 0; i < 8; i++ {
				go func() {
					rows, err := cache.query(ctx, "SELECT value", nil, true, func() (driver.Rows, error) { calls.Add(1); return &testRows{n: 1}, nil })
					if rows != nil {
						rows.Close()
					}
					finished <- err
				}()
			}
			deadline := time.Now().Add(time.Second)
			for cache.Stats().Shared < 8 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			shared := cache.Stats().Shared
			if cancelLeader {
				cancel()
			}
			close(release)
			failures := 0
			for i := 0; i < 9; i++ {
				if err := <-finished; err != nil {
					failures++
				}
			}
			if shared != 8 {
				t.Fatalf("waiters did not join: %d", shared)
			}
			wantCalls, wantFailures := int32(1), 9
			if cancelLeader {
				wantCalls, wantFailures = 2, 1
			}
			if calls.Load() != wantCalls || failures != wantFailures {
				t.Fatalf("calls=%d failures=%d; want calls=%d failures=%d", calls.Load(), failures, wantCalls, wantFailures)
			}
		})
	}
}
