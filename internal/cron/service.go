package cron

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/adhocore/gronx"
)

// Service manages cron jobs with persistence, scheduling, and execution.
type Service struct {
	storePath string
	store     Store
	onJob     JobHandler
	running   bool
	stopChan  chan struct{}
	mu        sync.Mutex
	runLog    []RunLogEntry // in-memory run history (last 200 entries)
	retryCfg  RetryConfig   // retry config for failed jobs
}

// NewService creates a new cron service.
// storePath is the path to the JSON file for job persistence.
// onJob is the callback invoked when a job fires (can be set later via SetOnJob).
func NewService(storePath string, onJob JobHandler) *Service {
	return &Service{
		storePath: storePath,
		store:     Store{Version: 1},
		onJob:     onJob,
		retryCfg:  DefaultRetryConfig(),
	}
}

// SetRetryConfig overrides the default retry configuration.
func (cs *Service) SetRetryConfig(cfg RetryConfig) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.retryCfg = cfg
}

// SetOnJob sets the job execution callback.
func (cs *Service) SetOnJob(handler JobHandler) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.onJob = handler
}

// Start loads persisted jobs and begins the scheduling loop.
func (cs *Service) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.running {
		return nil
	}

	if err := cs.loadUnsafe(); err != nil {
		slog.Warn("cron: failed to load store, starting fresh", "error", err)
		cs.store = Store{Version: 1}
	}

	// Compute next runs for all enabled jobs
	now := nowMS()
	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.Enabled && job.State.NextRunAtMS == nil {
			next := cs.computeNextRun(&job.Schedule, now)
			job.State.NextRunAtMS = next
		}
	}
	cs.saveUnsafe()

	cs.stopChan = make(chan struct{})
	cs.running = true

	go cs.runLoop(cs.stopChan)

	slog.Info("cron service started", "jobs", len(cs.store.Jobs))
	return nil
}

// Stop halts the scheduling loop.
func (cs *Service) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.running {
		return
	}

	close(cs.stopChan)
	cs.running = false
	slog.Info("cron service stopped")
}

// AddJob creates and registers a new cron job.
func (cs *Service) AddJob(name string, schedule Schedule, message string, deliver bool, channel, to, agentID string) (*Job, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Validate schedule
	if err := cs.validateSchedule(&schedule); err != nil {
		return nil, fmt.Errorf("invalid schedule: %w", err)
	}

	now := nowMS()
	job := Job{
		ID:      generateID(),
		Name:    name,
		AgentID: agentID,
		Enabled: true,
		Schedule: schedule,
		Payload: Payload{
			Kind:    "agent_turn",
			Message: message,
			Deliver: deliver,
			Channel: channel,
			To:      to,
		},
		CreatedAtMS:    now,
		UpdatedAtMS:    now,
		DeleteAfterRun: schedule.Kind == "at",
	}

	next := cs.computeNextRun(&job.Schedule, now)
	job.State.NextRunAtMS = next

	cs.store.Jobs = append(cs.store.Jobs, job)
	cs.saveUnsafe()

	slog.Info("cron job added", "id", job.ID, "name", name, "kind", schedule.Kind)
	return &job, nil
}

// RemoveJob deletes a job by ID.
func (cs *Service) RemoveJob(jobID string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, job := range cs.store.Jobs {
		if job.ID == jobID {
			cs.store.Jobs = append(cs.store.Jobs[:i], cs.store.Jobs[i+1:]...)
			cs.saveUnsafe()
			slog.Info("cron job removed", "id", jobID)
			return nil
		}
	}
	return fmt.Errorf("job %s not found", jobID)
}

// EnableJob toggles a job's enabled state.
func (cs *Service) EnableJob(jobID string, enabled bool) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			cs.store.Jobs[i].Enabled = enabled
			cs.store.Jobs[i].UpdatedAtMS = nowMS()
			if enabled {
				next := cs.computeNextRun(&cs.store.Jobs[i].Schedule, nowMS())
				cs.store.Jobs[i].State.NextRunAtMS = next
			} else {
				cs.store.Jobs[i].State.NextRunAtMS = nil
			}
			cs.saveUnsafe()
			slog.Info("cron job toggled", "id", jobID, "enabled", enabled)
			return nil
		}
	}
	return fmt.Errorf("job %s not found", jobID)
}

// ListJobs returns all jobs, optionally including disabled ones.
func (cs *Service) ListJobs(includeDisabled bool) []Job {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var result []Job
	for _, job := range cs.store.Jobs {
		if includeDisabled || job.Enabled {
			result = append(result, job)
		}
	}
	return result
}

// GetJob returns a job by ID.
func (cs *Service) GetJob(jobID string) (*Job, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i, job := range cs.store.Jobs {
		if job.ID == jobID {
			return &cs.store.Jobs[i], true
		}
	}
	return nil, false
}

// UpdateJob patches an existing job's fields.
// Matching TS cron.update — only non-zero/non-nil fields are applied.
func (cs *Service) UpdateJob(jobID string, patch JobPatch) (*Job, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID != jobID {
			continue
		}
		job := &cs.store.Jobs[i]

		if patch.Name != "" {
			job.Name = patch.Name
		}
		if patch.AgentID != nil {
			job.AgentID = *patch.AgentID
		}
		if patch.Enabled != nil {
			job.Enabled = *patch.Enabled
		}
		if patch.Schedule != nil {
			if err := cs.validateSchedule(patch.Schedule); err != nil {
				return nil, fmt.Errorf("invalid schedule: %w", err)
			}
			job.Schedule = *patch.Schedule
		}
		if patch.Message != "" {
			job.Payload.Message = patch.Message
		}
		if patch.Deliver != nil {
			job.Payload.Deliver = *patch.Deliver
		}
		if patch.Channel != nil {
			job.Payload.Channel = *patch.Channel
		}
		if patch.To != nil {
			job.Payload.To = *patch.To
		}
		if patch.DeleteAfterRun != nil {
			job.DeleteAfterRun = *patch.DeleteAfterRun
		}

		job.UpdatedAtMS = nowMS()

		// Recompute next run if schedule or enabled changed
		if job.Enabled {
			next := cs.computeNextRun(&job.Schedule, nowMS())
			job.State.NextRunAtMS = next
		} else {
			job.State.NextRunAtMS = nil
		}

		cs.saveUnsafe()
		slog.Info("cron job updated", "id", jobID)
		result := cs.store.Jobs[i] // copy
		return &result, nil
	}
	return nil, fmt.Errorf("job %s not found", jobID)
}

// RunJob manually triggers a job execution.
// mode: "force" = run regardless of schedule, "due" = only run if due.
// Returns (ran bool, reason string, error).
func (cs *Service) RunJob(jobID string, force bool) (bool, string, error) {
	cs.mu.Lock()

	var job *Job
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			j := cs.store.Jobs[i] // copy
			job = &j
			break
		}
	}
	handler := cs.onJob
	cs.mu.Unlock()

	if job == nil {
		return false, "", fmt.Errorf("job %s not found", jobID)
	}
	if handler == nil {
		return false, "", fmt.Errorf("no job handler configured")
	}

	if !force {
		// Check if job is due
		if job.State.NextRunAtMS == nil || *job.State.NextRunAtMS > nowMS() {
			return false, "not-due", nil
		}
	}

	// Execute outside lock with retry
	slog.Info("cron manual run", "id", job.ID, "name", job.Name, "force", force)
	result, _, err := ExecuteWithRetry(func() (string, error) {
		return handler(job)
	}, cs.retryCfg)

	// Update state
	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID != jobID {
			continue
		}
		now := nowMS()
		cs.store.Jobs[i].State.LastRunAtMS = &now
		if err != nil {
			cs.store.Jobs[i].State.LastStatus = "error"
			cs.store.Jobs[i].State.LastError = err.Error()
		} else {
			cs.store.Jobs[i].State.LastStatus = "ok"
			cs.store.Jobs[i].State.LastError = ""
		}

		// Recompute next run (unless one-time and delete after run)
		if cs.store.Jobs[i].DeleteAfterRun {
			cs.store.Jobs = append(cs.store.Jobs[:i], cs.store.Jobs[i+1:]...)
		} else {
			next := cs.computeNextRun(&cs.store.Jobs[i].Schedule, now)
			cs.store.Jobs[i].State.NextRunAtMS = next
		}
		cs.saveUnsafe()
		break
	}

	// Record run log
	cs.recordRun(jobID, err, result)

	if err != nil {
		return true, "", err
	}
	return true, result, nil
}

// GetRunLog returns recent run log entries for a job (or all jobs if jobID is empty).
func (cs *Service) GetRunLog(jobID string, limit int) []RunLogEntry {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if limit <= 0 {
		limit = 20
	}

	var result []RunLogEntry
	for i := len(cs.runLog) - 1; i >= 0 && len(result) < limit; i-- {
		entry := cs.runLog[i]
		if jobID == "" || entry.JobID == jobID {
			result = append(result, entry)
		}
	}
	return result
}

func (cs *Service) recordRun(jobID string, err error, resultText string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry := RunLogEntry{
		Ts:    nowMS(),
		JobID: jobID,
	}
	if err != nil {
		entry.Status = "error"
		entry.Error = err.Error()
	} else {
		entry.Status = "ok"
		entry.Summary = TruncateOutput(resultText)
	}

	cs.runLog = append(cs.runLog, entry)
	// Keep last 200 entries in memory
	if len(cs.runLog) > 200 {
		cs.runLog = cs.runLog[len(cs.runLog)-200:]
	}
}

// Status returns the service status.
func (cs *Service) Status() map[string]interface{} {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	return map[string]interface{}{
		"enabled":      cs.running,
		"jobs":         len(cs.store.Jobs),
		"nextWakeAtMs": cs.getNextWakeMS(),
	}
}

// --- Internal scheduling loop ---

func (cs *Service) runLoop(stopChan chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			cs.checkJobs()
		}
	}
}

func (cs *Service) checkJobs() {
	cs.mu.Lock()

	now := nowMS()
	var dueJobIDs []string

	for i := range cs.store.Jobs {
		job := &cs.store.Jobs[i]
		if job.Enabled && job.State.NextRunAtMS != nil && *job.State.NextRunAtMS <= now {
			dueJobIDs = append(dueJobIDs, job.ID)
		}
	}

	if len(dueJobIDs) == 0 {
		cs.mu.Unlock()
		return
	}

	// Clear NextRunAtMS to prevent duplicate execution
	dueMap := make(map[string]bool, len(dueJobIDs))
	for _, id := range dueJobIDs {
		dueMap[id] = true
	}
	for i := range cs.store.Jobs {
		if dueMap[cs.store.Jobs[i].ID] {
			cs.store.Jobs[i].State.NextRunAtMS = nil
		}
	}
	cs.saveUnsafe()
	cs.mu.Unlock()

	// Execute jobs outside lock
	for _, jobID := range dueJobIDs {
		cs.executeJobByID(jobID)
	}
}

func (cs *Service) executeJobByID(jobID string) {
	cs.mu.Lock()
	var job *Job
	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID == jobID {
			j := cs.store.Jobs[i] // copy
			job = &j
			break
		}
	}
	handler := cs.onJob
	cs.mu.Unlock()

	if job == nil || handler == nil {
		return
	}

	slog.Info("cron executing job", "id", job.ID, "name", job.Name)

	result, attempts, err := ExecuteWithRetry(func() (string, error) {
		return handler(job)
	}, cs.retryCfg)

	if attempts > 1 {
		slog.Info("cron job retried", "id", job.ID, "attempts", attempts, "success", err == nil)
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()

	for i := range cs.store.Jobs {
		if cs.store.Jobs[i].ID != jobID {
			continue
		}

		now := nowMS()
		cs.store.Jobs[i].State.LastRunAtMS = &now

		if err != nil {
			cs.store.Jobs[i].State.LastStatus = "error"
			cs.store.Jobs[i].State.LastError = err.Error()
			slog.Error("cron job failed", "id", jobID, "error", err)
		} else {
			cs.store.Jobs[i].State.LastStatus = "ok"
			cs.store.Jobs[i].State.LastError = ""
			slog.Info("cron job completed", "id", jobID, "result", result)
		}

		// Schedule next run or handle one-time jobs
		if cs.store.Jobs[i].DeleteAfterRun {
			cs.store.Jobs = append(cs.store.Jobs[:i], cs.store.Jobs[i+1:]...)
		} else {
			next := cs.computeNextRun(&cs.store.Jobs[i].Schedule, now)
			cs.store.Jobs[i].State.NextRunAtMS = next
			if next == nil {
				cs.store.Jobs[i].Enabled = false
			}
		}
		break
	}

	cs.saveUnsafe()
}

// --- Schedule computation ---

func (cs *Service) computeNextRun(schedule *Schedule, now int64) *int64 {
	switch schedule.Kind {
	case "at":
		if schedule.AtMS != nil && *schedule.AtMS > now {
			return schedule.AtMS
		}
		return nil

	case "every":
		if schedule.EveryMS == nil || *schedule.EveryMS <= 0 {
			return nil
		}
		next := now + *schedule.EveryMS
		return &next

	case "cron":
		if schedule.Expr == "" {
			return nil
		}
		nowTime := time.UnixMilli(now)
		nextTime, err := gronx.NextTickAfter(schedule.Expr, nowTime, false)
		if err != nil {
			slog.Error("cron: failed to compute next run", "expr", schedule.Expr, "error", err)
			return nil
		}
		nextMS := nextTime.UnixMilli()
		return &nextMS

	default:
		return nil
	}
}

func (cs *Service) validateSchedule(schedule *Schedule) error {
	switch schedule.Kind {
	case "at":
		if schedule.AtMS == nil {
			return fmt.Errorf("at schedule requires atMs")
		}
	case "every":
		if schedule.EveryMS == nil || *schedule.EveryMS <= 0 {
			return fmt.Errorf("every schedule requires positive everyMs")
		}
	case "cron":
		if schedule.Expr == "" {
			return fmt.Errorf("cron schedule requires expr")
		}
		gx := gronx.New()
		if !gx.IsValid(schedule.Expr) {
			return fmt.Errorf("invalid cron expression: %s", schedule.Expr)
		}
	default:
		return fmt.Errorf("unknown schedule kind: %s", schedule.Kind)
	}
	return nil
}

func (cs *Service) getNextWakeMS() *int64 {
	var earliest *int64
	for _, job := range cs.store.Jobs {
		if job.Enabled && job.State.NextRunAtMS != nil {
			if earliest == nil || *job.State.NextRunAtMS < *earliest {
				earliest = job.State.NextRunAtMS
			}
		}
	}
	return earliest
}

// --- Persistence ---

func (cs *Service) loadUnsafe() error {
	data, err := os.ReadFile(cs.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &cs.store)
}

func (cs *Service) saveUnsafe() error {
	dir := filepath.Dir(cs.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cs.store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.storePath, data, 0644)
}
