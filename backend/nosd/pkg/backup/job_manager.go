package backup

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// JobManager manages backup jobs
type JobManager struct {
	logger zerolog.Logger
	jobs   map[string]*BackupJob
	mu     sync.RWMutex
}

// NewJobManager creates a new job manager
func NewJobManager(logger zerolog.Logger) *JobManager {
	return &JobManager{
		logger: logger.With().Str("component", "job-manager").Logger(),
		jobs:   make(map[string]*BackupJob),
	}
}

// AddJob adds a new job
func (jm *JobManager) AddJob(job *BackupJob) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	jm.jobs[job.ID] = job.clone()
	jm.logger.Info().
		Str("id", job.ID).
		Str("type", job.Type).
		Msg("Job added")
}

// UpdateJob updates an existing job
// UpdateJob stores a copy of job. The job's runner keeps its own copy (jobs
// used to be one shared pointer that runners wrote while readers read it);
// the log stays the stored one (AddLogEntry), and a canceled job stays
// canceled.
func (jm *JobManager) UpdateJob(job *BackupJob) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	next := job.clone()
	if cur, ok := jm.jobs[job.ID]; ok {
		next.LogEntries = cur.LogEntries
		if cur.State == JobStateCanceled {
			next.State, next.FinishedAt = cur.State, cur.FinishedAt
		}
	}
	jm.jobs[job.ID] = next
}

// clone copies a job so the copy shares no slices with it.
func (j *BackupJob) clone() *BackupJob {
	c := *j
	c.LogEntries = append([]LogEntry(nil), j.LogEntries...)
	c.Subvolumes = append([]string(nil), j.Subvolumes...)
	return &c
}

// GetJob returns a job by ID
func (jm *JobManager) GetJob(id string) (*BackupJob, bool) {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	job, ok := jm.jobs[id]
	if !ok {
		return nil, false
	}
	return job.clone(), true
}

// ListJobs returns all jobs
func (jm *JobManager) ListJobs() []*BackupJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job.clone())
	}

	return jobs
}

// ListRecentJobs returns recent jobs
func (jm *JobManager) ListRecentJobs(limit int) []*BackupJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()

	// Get all jobs
	jobs := make([]*BackupJob, 0, len(jm.jobs))
	for _, job := range jm.jobs {
		jobs = append(jobs, job)
	}

	// Sort by start time (newest first)
	for i := 0; i < len(jobs)-1; i++ {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[j].StartedAt.After(jobs[i].StartedAt) {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}

	// Return limited results
	if limit > 0 && limit < len(jobs) {
		return jobs[:limit]
	}

	return jobs
}

// CancelJob cancels a running job
func (jm *JobManager) CancelJob(id string) error {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[id]
	if !ok {
		return nil
	}

	if job.State == JobStateRunning || job.State == JobStatePending {
		job.State = JobStateCanceled
		now := time.Now()
		job.FinishedAt = &now
		jm.logger.Info().Str("id", id).Msg("Job canceled")
	}

	return nil
}

// CleanupOldJobs removes completed jobs older than the specified duration
func (jm *JobManager) CleanupOldJobs(maxAge time.Duration) int {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	now := time.Now()
	deleted := 0

	for id, job := range jm.jobs {
		// Only clean up completed jobs
		if job.State != JobStateSucceeded && job.State != JobStateFailed && job.State != JobStateCanceled {
			continue
		}

		// Check age
		if job.FinishedAt != nil && now.Sub(*job.FinishedAt) > maxAge {
			delete(jm.jobs, id)
			deleted++
		}
	}

	if deleted > 0 {
		jm.logger.Info().Int("count", deleted).Msg("Cleaned up old jobs")
	}

	return deleted
}

// AddLogEntry adds a log entry to a job
func (jm *JobManager) AddLogEntry(jobID string, level string, message string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	job.LogEntries = append(job.LogEntries, entry)

	// Keep only last 100 entries
	if len(job.LogEntries) > 100 {
		job.LogEntries = job.LogEntries[len(job.LogEntries)-100:]
	}
}

// UpdateProgress updates job progress
func (jm *JobManager) UpdateProgress(jobID string, progress int, bytesTotal, bytesDone int64) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	job, ok := jm.jobs[jobID]
	if !ok {
		return
	}

	job.Progress = progress
	job.BytesTotal = bytesTotal
	job.BytesDone = bytesDone
}
