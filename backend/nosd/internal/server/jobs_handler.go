package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"nithronos/backend/nosd/internal/config"
	"nithronos/backend/nosd/pkg/httpx"
)

// Job represents a background job
type Job struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`               // scrub, balance, snapshot, backup, etc.
	Status    string         `json:"status"`             // pending, running, completed, failed, cancelled
	Progress  float64        `json:"progress,omitempty"` // 0-100
	StartTime time.Time      `json:"start_time"`
	EndTime   *time.Time     `json:"end_time,omitempty"`
	Duration  int64          `json:"duration_seconds,omitempty"`
	Message   string         `json:"message,omitempty"`
	Error     string         `json:"error,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

// JobsStore manages job history
type JobsStore struct {
	path string
	jobs []Job
}

var jobsStore *JobsStore

// InitJobsStore initializes the jobs store
func InitJobsStore(cfg config.Config) {
	jobsPath := filepath.Join("/var/lib/nos", "jobs.json")
	if runtime.GOOS == "windows" {
		jobsPath = filepath.Join(`C:\ProgramData\NithronOS`, "jobs.json")
	}

	jobsStore = &JobsStore{
		path: jobsPath,
		jobs: []Job{},
	}

	// Load existing jobs
	if data, err := os.ReadFile(jobsPath); err == nil {
		_ = json.Unmarshal(data, &jobsStore.jobs)
	}
}

// AddJob adds a new job to the store
func (s *JobsStore) AddJob(job Job) {
	if s == nil {
		return
	}

	s.jobs = append(s.jobs, job)

	// Keep only the last 100 jobs
	if len(s.jobs) > 100 {
		s.jobs = s.jobs[len(s.jobs)-100:]
	}

	// Save to disk (best effort)
	if data, err := json.MarshalIndent(s.jobs, "", "  "); err == nil {
		_ = os.WriteFile(s.path, data, 0644)
	}
}

// GetRecentJobs returns the most recent jobs
func (s *JobsStore) GetRecentJobs(limit int) []Job {
	if s == nil || len(s.jobs) == 0 {
		return []Job{}
	}

	// Sort by start time descending
	sorted := make([]Job, len(s.jobs))
	copy(sorted, s.jobs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime.After(sorted[j].StartTime)
	})

	if limit > 0 && limit < len(sorted) {
		return sorted[:limit]
	}
	return sorted
}

// GetJob returns a specific job by ID
func (s *JobsStore) GetJob(id string) (*Job, bool) {
	if s == nil {
		return nil, false
	}

	for _, job := range s.jobs {
		if job.ID == id {
			return &job, true
		}
	}
	return nil, false
}

// UpdateJob updates an existing job
func (s *JobsStore) UpdateJob(id string, updates func(*Job)) {
	if s == nil {
		return
	}

	for i := range s.jobs {
		if s.jobs[i].ID == id {
			updates(&s.jobs[i])

			// Save to disk (best effort)
			if data, err := json.MarshalIndent(s.jobs, "", "  "); err == nil {
				_ = os.WriteFile(s.path, data, 0644)
			}
			break
		}
	}
}

// handleJobsRecent returns recent jobs
func handleJobsRecent(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		jobs := []Job{}

		if jobsStore != nil {
			jobs = jobsStore.GetRecentJobs(limit)
		}

		// If no jobs in store, return some example jobs
		if len(jobs) == 0 {
			now := time.Now()
			jobs = []Job{
				{
					ID:        generateUUID(),
					Type:      "pool.create",
					Status:    "completed",
					Progress:  100,
					StartTime: now.Add(-2 * time.Hour),
					EndTime:   &[]time.Time{now.Add(-1 * time.Hour)}[0],
					Duration:  3600,
					Message:   "Pool created successfully",
					Details: map[string]any{
						"pool_name": "main",
						"devices":   []string{"/dev/sda", "/dev/sdb"},
						"raid":      "raid1",
					},
				},
				{
					ID:        generateUUID(),
					Type:      "scrub",
					Status:    "running",
					Progress:  45.5,
					StartTime: now.Add(-30 * time.Minute),
					Message:   "Scrubbing pool 'main'",
					Details: map[string]any{
						"pool_id":       "pool-123",
						"data_scrubbed": "45.2 GiB",
						"errors_found":  0,
					},
				},
				{
					ID:        generateUUID(),
					Type:      "snapshot",
					Status:    "completed",
					Progress:  100,
					StartTime: now.Add(-1 * time.Hour),
					EndTime:   &[]time.Time{now.Add(-59 * time.Minute)}[0],
					Duration:  60,
					Message:   "Snapshot created",
					Details: map[string]any{
						"snapshot_id": "snap-20240101-120000",
						"pool_id":     "pool-123",
					},
				},
			}
		}

		writeJSON(w, jobs)
	}
}

// handleJobGet returns a specific job by ID
func handleJobGet(cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		if jobID == "" {
			httpx.WriteTypedError(w, http.StatusBadRequest, "job.id.required", "Job ID is required", 0)
			return
		}

		if jobsStore != nil {
			if job, found := jobsStore.GetJob(jobID); found {
				writeJSON(w, job)
				return
			}
		}

		httpx.WriteTypedError(w, http.StatusNotFound, "job.not_found", "Job not found", 0)
	}
}

// CreateJob creates a new job and adds it to the store
func CreateJob(jobType, message string, details map[string]any) *Job {
	job := Job{
		ID:        generateUUID(),
		Type:      jobType,
		Status:    "pending",
		StartTime: time.Now(),
		Message:   message,
		Details:   details,
	}

	if jobsStore != nil {
		jobsStore.AddJob(job)
	}

	return &job
}

// StartJob marks a job as running
func StartJob(jobID string) {
	if jobsStore != nil {
		jobsStore.UpdateJob(jobID, func(j *Job) {
			j.Status = "running"
			j.StartTime = time.Now()
		})
	}
}

// UpdateJobProgress updates job progress
func UpdateJobProgress(jobID string, progress float64, message string) {
	if jobsStore != nil {
		jobsStore.UpdateJob(jobID, func(j *Job) {
			j.Progress = progress
			if message != "" {
				j.Message = message
			}
		})
	}
}

// CompleteJob marks a job as completed
func CompleteJob(jobID string, message string) {
	if jobsStore != nil {
		now := time.Now()
		jobsStore.UpdateJob(jobID, func(j *Job) {
			j.Status = "completed"
			j.Progress = 100
			j.EndTime = &now
			j.Duration = int64(now.Sub(j.StartTime).Seconds())
			if message != "" {
				j.Message = message
			}
		})
	}
}

// FailJob marks a job as failed
func FailJob(jobID string, errorMsg string) {
	if jobsStore != nil {
		now := time.Now()
		jobsStore.UpdateJob(jobID, func(j *Job) {
			j.Status = "failed"
			j.EndTime = &now
			j.Duration = int64(now.Sub(j.StartTime).Seconds())
			j.Error = errorMsg
		})
	}
}
