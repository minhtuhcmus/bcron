package better_cron

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// JobStatus represents the current state of a job
type JobStatus int

const (
	StatusIdle JobStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

// JobMetadata contains information about a job execution
type JobMetadata struct {
	ID        cron.EntryID
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Status    JobStatus
	Error     error
}

// EnhancedCron wraps the standard better_cron scheduler with additional features
type EnhancedCron struct {
	cron           *cron.Cron
	activeJobs     sync.Map
	shutdownCtx    context.Context
	cancelShutdown context.CancelFunc
	timeout        time.Duration
	logger         Logger
}

// Logger interface for custom logging
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// NewEnhancedCron creates a new instance of EnhancedCron
func NewEnhancedCron(opts ...Option) *EnhancedCron {
	ctx, cancel := context.WithCancel(context.Background())
	ec := &EnhancedCron{
		cron:           cron.New(cron.WithSeconds()),
		shutdownCtx:    ctx,
		cancelShutdown: cancel,
		timeout:        30 * time.Second, // Default timeout
	}

	// Apply options
	for _, opt := range opts {
		opt(ec)
	}

	return ec
}

// Option represents configuration options for EnhancedCron
type Option func(*EnhancedCron)

// WithTimeout sets the shutdown timeout
func WithTimeout(timeout time.Duration) Option {
	return func(ec *EnhancedCron) {
		ec.timeout = timeout
	}
}

// WithLogger sets a custom logger
func WithLogger(logger Logger) Option {
	return func(ec *EnhancedCron) {
		ec.logger = logger
	}
}

// AddJob adds a new job with enhanced wrapping
func (ec *EnhancedCron) AddJob(spec string, job cron.Job, name string) (cron.EntryID, error) {
	wrappedJob := ec.wrapJob(job, name)
	return ec.cron.AddJob(spec, wrappedJob)
}

// In the wrapJob function, modify the job execution:
func (ec *EnhancedCron) wrapJob(job cron.Job, name string) cron.Job {
	return cron.FuncJob(func() {
		// Create job-specific context with timeout
		jobCtx, cancel := context.WithTimeout(ec.shutdownCtx, ec.timeout)
		defer cancel()

		metadata := &JobMetadata{
			Name:      name,
			StartTime: time.Now(),
			Status:    StatusRunning,
		}

		// Create a WaitGroup for this specific job
		var wg sync.WaitGroup
		wg.Add(1)

		// Store active job with the WaitGroup
		jobInfo := struct {
			metadata *JobMetadata
			wg       *sync.WaitGroup
		}{metadata, &wg}

		ec.activeJobs.Store(name, jobInfo)
		defer ec.activeJobs.Delete(name)

		// Run job in goroutine
		go func() {
			defer wg.Done() // Signal completion
			defer func() {
				if r := recover(); r != nil {
					metadata.Status = StatusFailed
					metadata.Error = fmt.Errorf("job panic: %v", r)
					// Log panic
				}
			}()

			job.Run()
			metadata.Status = StatusCompleted
		}()

		// Wait for either job completion or context cancellation
		select {
		case <-jobCtx.Done():
			// Wait for job to actually finish even after cancellation
			wg.Wait()
			metadata.Status = StatusCancelled
			metadata.Error = jobCtx.Err()
		case <-waitWithTimeout(&wg, ec.timeout):
			// Job completed normally
		}

		metadata.EndTime = time.Now()
	})
}

// Helper function to wait with timeout
func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}

// Start starts the better_cron scheduler
func (ec *EnhancedCron) Start() {
	ec.cron.Start()
}

// Then modify the Shutdown method:
func (ec *EnhancedCron) Shutdown() error {
	// Signal shutdown to all jobs
	ec.cancelShutdown()

	// Create timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), ec.timeout)
	defer cancel()

	// Stop accepting new jobs
	stopCtx := ec.cron.Stop()

	// Create a WaitGroup for all jobs
	var wg sync.WaitGroup

	// Wait for all jobs to actually complete
	ec.activeJobs.Range(func(key, value interface{}) bool {
		jobInfo := value.(struct {
			metadata *JobMetadata
			wg       *sync.WaitGroup
		})
		wg.Add(1)
		go func(jobWg *sync.WaitGroup) {
			defer wg.Done()
			// Wait for this specific job to complete
			jobWg.Wait()
		}(jobInfo.wg)
		return true
	})

	// Wait for all components
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()        // Wait for all jobs to complete
		<-stopCtx.Done() // Wait for cron to stop
	}()

	// Wait for shutdown completion or timeout
	select {
	case <-shutdownCtx.Done():
		return fmt.Errorf("shutdown timed out after %v", ec.timeout)
	case <-done:
		return nil
	}
}

// GetJobStatus returns the current status of a job by name
func (ec *EnhancedCron) GetJobStatus(name string) (*JobMetadata, bool) {
	if value, ok := ec.activeJobs.Load(name); ok {
		return value.(*JobMetadata), true
	}
	return nil, false
}

// GetActiveJobs returns a list of all currently running jobs
func (ec *EnhancedCron) GetActiveJobs() []*JobMetadata {
	var jobs []*JobMetadata
	ec.activeJobs.Range(func(key, value interface{}) bool {
		metadata := value.(*JobMetadata)
		if metadata.Status == StatusRunning {
			jobs = append(jobs, metadata)
		}
		return true
	})
	return jobs
}
