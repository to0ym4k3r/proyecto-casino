package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// spyExpiryRunner records calls to ExpireStaleBonuses.
type spyExpiryRunner struct {
	callCount atomic.Int32
	lastErr   error
}

func (s *spyExpiryRunner) ExpireStaleBonuses(_ context.Context) (int, error) {
	s.callCount.Add(1)
	return 0, s.lastErr
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

func TestScheduler_CallsExpireOnStart(t *testing.T) {
	spy := &spyExpiryRunner{}
	s := NewBonusExpiryScheduler(spy, time.Hour, testLogger())
	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(50 * time.Millisecond)

	if n := spy.callCount.Load(); n < 1 {
		t.Errorf("callCount = %d, want >= 1", n)
	}
}

func TestScheduler_CallsExpirePeriodically(t *testing.T) {
	spy := &spyExpiryRunner{}
	s := NewBonusExpiryScheduler(spy, 50*time.Millisecond, testLogger())
	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(250 * time.Millisecond)

	if n := spy.callCount.Load(); n < 3 {
		t.Errorf("callCount = %d, want >= 3 (initial + at least 2 ticks)", n)
	}
}

func TestScheduler_StopPreventsMoreCalls(t *testing.T) {
	spy := &spyExpiryRunner{}
	s := NewBonusExpiryScheduler(spy, 50*time.Millisecond, testLogger())
	s.Start(context.Background())

	time.Sleep(120 * time.Millisecond)
	s.Stop()
	stopped := spy.callCount.Load()

	time.Sleep(150 * time.Millisecond)

	if n := spy.callCount.Load(); n != stopped {
		t.Errorf("callCount after stop = %d, want %d (no more calls)", n, stopped)
	}
}

func TestScheduler_ContinuesOnError(t *testing.T) {
	spy := &spyExpiryRunner{lastErr: errors.New("db down")}
	s := NewBonusExpiryScheduler(spy, 50*time.Millisecond, testLogger())
	s.Start(context.Background())
	defer s.Stop()

	time.Sleep(250 * time.Millisecond)

	if n := spy.callCount.Load(); n < 2 {
		t.Errorf("callCount = %d, want >= 2 (scheduler must keep ticking after error)", n)
	}
}
