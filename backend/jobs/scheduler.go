package jobs

import (
	"fmt"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// JobStatus holds the runtime state of a registered background job.
type JobStatus struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Schedule    string    `json:"schedule"` // human-readable, e.g. "Monday 03:01 AM"
	IsRunning   bool      `json:"isRunning"`
	LastRun     time.Time `json:"lastRun"` // zero value if never run
	NextRun     time.Time `json:"nextRun"`
	LastError   string    `json:"lastError"` // empty if last run succeeded
}

type job struct {
	JobStatus
	weekday time.Weekday
	hour    int
	minute  int
	runFn   func()
	mu      sync.Mutex
}

var (
	registry   []*job
	registryMu sync.RWMutex
)

// Register adds a job to the scheduler.
// weekday: time.Monday … time.Sunday
// hour/minute: local time for weekly execution
func Register(name, description string, weekday time.Weekday, hour, minute int, runFn func()) {
	j := &job{
		JobStatus: JobStatus{
			Name:        name,
			Description: description,
			Schedule:    fmt.Sprintf("%s %02d:%02d AM", weekday, hour, minute),
		},
		weekday: weekday,
		hour:    hour,
		minute:  minute,
		runFn:   runFn,
	}
	j.NextRun = nextWeekdayTime(weekday, hour, minute)

	registryMu.Lock()
	registry = append(registry, j)
	registryMu.Unlock()
}

// StartAll launches goroutines for all registered jobs.
// No job runs immediately at startup — each waits until its scheduled weekday/time.
func StartAll() {
	registryMu.RLock()
	defer registryMu.RUnlock()
	for _, j := range registry {
		go runLoop(j)
	}
}

// RunNow triggers a job immediately by name.
// Returns an error if the job is already running or not found.
func RunNow(name string) error {
	registryMu.RLock()
	j := findJob(name)
	registryMu.RUnlock()

	if j == nil {
		return fmt.Errorf("job '%s' not found", name)
	}

	j.mu.Lock()
	if j.IsRunning {
		j.mu.Unlock()
		return fmt.Errorf("job '%s' is already running", name)
	}
	j.IsRunning = true
	j.mu.Unlock()

	go func() {
		defer func() {
			j.mu.Lock()
			j.IsRunning = false
			j.LastRun = time.Now()
			j.NextRun = nextWeekdayTime(j.weekday, j.hour, j.minute)
			j.mu.Unlock()
		}()
		logger.Infof("Jobs: Manual run started for '%s'", name)
		j.runFn()
		logger.Infof("Jobs: Manual run completed for '%s'", name)
	}()

	return nil
}

// GetAll returns a snapshot of all job statuses.
func GetAll() []JobStatus {
	registryMu.RLock()
	defer registryMu.RUnlock()

	result := make([]JobStatus, 0, len(registry))
	for _, j := range registry {
		j.mu.Lock()
		snap := j.JobStatus // copy
		j.mu.Unlock()
		result = append(result, snap)
	}
	return result
}

// runLoop is the goroutine for scheduled execution of a job.
func runLoop(j *job) {
	for {
		waitDur := time.Until(j.NextRun)
		if waitDur > 0 {
			logger.Infof("Jobs: '%s' scheduled — waiting %s until %s",
				j.Name, waitDur.Round(time.Minute), j.NextRun.Format("Mon Jan 2 15:04"))
			time.Sleep(waitDur)
		}

		j.mu.Lock()
		if j.IsRunning {
			// Another goroutine (RunNow) beat us to it — skip and reschedule
			j.NextRun = nextWeekdayTime(j.weekday, j.hour, j.minute)
			j.mu.Unlock()
			continue
		}
		j.IsRunning = true
		j.mu.Unlock()

		logger.Infof("Jobs: Scheduled run starting for '%s'", j.Name)
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("Jobs: '%s' panicked: %v", j.Name, r)
					j.mu.Lock()
					j.LastError = fmt.Sprintf("panic: %v", r)
					j.mu.Unlock()
				}
			}()
			j.runFn()
		}()

		j.mu.Lock()
		j.IsRunning = false
		j.LastRun = time.Now()
		j.NextRun = nextWeekdayTime(j.weekday, j.hour, j.minute)
		j.LastError = "" // clear on success
		j.mu.Unlock()

		logger.Infof("Jobs: Scheduled run complete for '%s'. Next: %s",
			j.Name, j.NextRun.Format("Mon Jan 2 15:04"))
	}
}

// nextWeekdayTime returns the time.Time of the next occurrence of weekday at hour:minute (local time).
// If that moment is in the past (or right now), it adds 7 days.
func nextWeekdayTime(weekday time.Weekday, hour, minute int) time.Time {
	now := time.Now()
	daysUntil := (int(weekday) - int(now.Weekday()) + 7) % 7
	candidate := time.Date(now.Year(), now.Month(), now.Day()+daysUntil, hour, minute, 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.Add(7 * 24 * time.Hour)
	}
	return candidate
}

// findJob looks up a job by name (caller must hold registryMu).
func findJob(name string) *job {
	for _, j := range registry {
		if j.Name == name {
			return j
		}
	}
	return nil
}
