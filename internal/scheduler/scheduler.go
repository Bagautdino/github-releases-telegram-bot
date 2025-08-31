package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Job represents a function to be executed by the scheduler
type Job func(ctx context.Context)

// Scheduler manages periodic execution of jobs
type Scheduler struct {
	logger     *slog.Logger
	interval   time.Duration
	job        Job
	ticker     *time.Ticker
	done       chan struct{}
	triggerCh  chan struct{}
}

// New creates a new scheduler instance
func New(logger *slog.Logger, interval time.Duration, job Job) *Scheduler {
	return &Scheduler{
		logger:    logger,
		interval:  interval,
		job:       job,
		done:      make(chan struct{}),
		triggerCh: make(chan struct{}, 1),
	}
}

// Start begins the scheduler execution
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("Starting scheduler", "interval", s.interval)

	s.ticker = time.NewTicker(s.interval)

	// Run job immediately on start
	go func() {
		s.logger.Info("Running initial job execution")
		s.executeJob(ctx)
	}()

	// Start periodic execution
	go s.run(ctx)
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.logger.Info("Stopping scheduler")

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.done)
}

// TriggerCheck manually triggers a job execution
func (s *Scheduler) TriggerCheck(ctx context.Context) error {
	select {
	case s.triggerCh <- struct{}{}:
		s.logger.Info("Manual trigger scheduled")
		return nil
	default:
		s.logger.Warn("Manual trigger ignored - already pending")
		return nil
	}
}

// run is the main scheduler loop
func (s *Scheduler) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Scheduler stopped due to context cancellation")
			return
		case <-s.done:
			s.logger.Info("Scheduler stopped")
			return
		case <-s.ticker.C:
			s.logger.Debug("Scheduler tick - executing job")
			go s.executeJob(ctx)
		case <-s.triggerCh:
			s.logger.Info("Manual trigger - executing job")
			go s.executeJob(ctx)
		}
	}
}

// executeJob runs the job with error handling and logging
func (s *Scheduler) executeJob(ctx context.Context) {
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Job panicked", "panic", r, "duration", time.Since(start))
		}
	}()

	s.logger.Debug("Job execution started")
	s.job(ctx)

	duration := time.Since(start)
	s.logger.Debug("Job execution completed", "duration", duration)
}

// Legacy function for backward compatibility
func Start(ctx context.Context, log *slog.Logger, interval time.Duration, job Job) {
	scheduler := New(log, interval, job)
	scheduler.Start(ctx)

	// Wait for context cancellation
	<-ctx.Done()
	scheduler.Stop()
}
