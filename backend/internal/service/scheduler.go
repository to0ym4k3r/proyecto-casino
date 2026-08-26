package service

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// ExpiryRunner is the subset of BonusService needed by the scheduler.
type ExpiryRunner interface {
	ExpireStaleBonuses(ctx context.Context) (int, error)
}

// BonusExpiryScheduler runs ExpireStaleBonuses on a fixed interval.
type BonusExpiryScheduler struct {
	runner ExpiryRunner
	interval time.Duration
	log      zerolog.Logger
	stopCh   chan struct{}
}

// NewBonusExpiryScheduler creates a scheduler that expires stale bonuses
// at the given interval. Call Start() to begin the background loop.
func NewBonusExpiryScheduler(runner ExpiryRunner, interval time.Duration, log zerolog.Logger) *BonusExpiryScheduler {
	return &BonusExpiryScheduler{
		runner:   runner,
		interval: interval,
		log:      log,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the background expiry loop. Safe to call only once.
func (s *BonusExpiryScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.expire(ctx)

		for {
			select {
			case <-ticker.C:
				s.expire(ctx)
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop signals the scheduler to shut down gracefully.
func (s *BonusExpiryScheduler) Stop() {
	close(s.stopCh)
}

func (s *BonusExpiryScheduler) expire(ctx context.Context) {
	count, err := s.runner.ExpireStaleBonuses(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("bonus expiry job failed")
		return
	}
	if count > 0 {
		s.log.Info().Int("expired", count).Msg("bonus expiry job completed")
	}
}
