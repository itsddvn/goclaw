package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
)

// QueueMode determines how incoming messages are handled when an agent
// is already processing a message for the same session.
type QueueMode string

const (
	// QueueModeQueue is simple FIFO: new messages wait until current finishes.
	QueueModeQueue QueueMode = "queue"

	// QueueModeFollowup queues as a follow-up after the current run completes.
	QueueModeFollowup QueueMode = "followup"

	// QueueModeInterrupt cancels the current run and starts the new message.
	QueueModeInterrupt QueueMode = "interrupt"
)

// DropPolicy determines which messages to drop when the queue is full.
type DropPolicy string

const (
	DropOld DropPolicy = "old" // drop oldest message
	DropNew DropPolicy = "new" // reject incoming message
)

// QueueConfig configures per-session message queuing.
type QueueConfig struct {
	Mode       QueueMode  `json:"mode"`
	Cap        int        `json:"cap"`
	Drop       DropPolicy `json:"drop"`
	DebounceMs int        `json:"debounce_ms"`
}

// DefaultQueueConfig returns sensible defaults.
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		Mode:       QueueModeQueue,
		Cap:        10,
		Drop:       DropOld,
		DebounceMs: 800,
	}
}

// RunFunc is the callback that executes an agent run.
// The scheduler calls this when it's the request's turn.
type RunFunc func(ctx context.Context, req agent.RunRequest) (*agent.RunResult, error)

// PendingRequest is a queued agent run awaiting execution.
type PendingRequest struct {
	Req      agent.RunRequest
	ResultCh chan RunOutcome
}

// RunOutcome is the result of a scheduled agent run.
type RunOutcome struct {
	Result *agent.RunResult
	Err    error
}

// SessionQueue serializes agent runs for a single session key.
// Only one run executes at a time; additional messages are queued.
type SessionQueue struct {
	key      string
	config   QueueConfig
	runFn    RunFunc
	laneMgr *LaneManager
	lane     string

	mu        sync.Mutex
	queue     []*PendingRequest
	active    bool               // whether a run is currently executing
	cancel    context.CancelFunc // cancel for the active run (interrupt mode)
	timer     *time.Timer        // debounce timer
	parentCtx context.Context    // stored from first Enqueue call, used for spawning runs
}

// NewSessionQueue creates a queue for a specific session.
func NewSessionQueue(key, lane string, cfg QueueConfig, laneMgr *LaneManager, runFn RunFunc) *SessionQueue {
	return &SessionQueue{
		key:      key,
		config:   cfg,
		runFn:    runFn,
		laneMgr:  laneMgr,
		lane:     lane,
	}
}

// Enqueue adds a request to the session queue.
// If no run is active, it starts immediately (after debounce).
// Returns a channel that receives the result when the run completes.
func (sq *SessionQueue) Enqueue(ctx context.Context, req agent.RunRequest) <-chan RunOutcome {
	outcome := make(chan RunOutcome, 1)
	pending := &PendingRequest{Req: req, ResultCh: outcome}

	sq.mu.Lock()
	defer sq.mu.Unlock()

	// Store parent context for spawning future runs
	if sq.parentCtx == nil {
		sq.parentCtx = ctx
	}

	switch sq.config.Mode {
	case QueueModeInterrupt:
		// Cancel current run if active
		if sq.active && sq.cancel != nil {
			sq.cancel()
		}
		// Clear existing queue and enqueue this one
		sq.drainQueue(RunOutcome{Err: context.Canceled})
		sq.queue = append(sq.queue, pending)
		if !sq.active {
			sq.scheduleNext(ctx)
		}

	default: // queue, followup
		if len(sq.queue) >= sq.config.Cap {
			sq.applyDropPolicy(pending)
		} else {
			sq.queue = append(sq.queue, pending)
		}

		if !sq.active {
			sq.scheduleNext(ctx)
		}
	}

	return outcome
}

// scheduleNext starts the next queued request, applying debounce.
// Must be called with sq.mu held.
func (sq *SessionQueue) scheduleNext(ctx context.Context) {
	if len(sq.queue) == 0 {
		return
	}

	debounce := time.Duration(sq.config.DebounceMs) * time.Millisecond
	if debounce <= 0 {
		sq.startNext(ctx)
		return
	}

	// Reset debounce timer: collapses rapid messages
	if sq.timer != nil {
		sq.timer.Stop()
	}
	sq.timer = time.AfterFunc(debounce, func() {
		sq.mu.Lock()
		defer sq.mu.Unlock()
		if !sq.active && len(sq.queue) > 0 {
			sq.startNext(ctx)
		}
	})
}

// startNext picks the first queued request and runs it in the lane.
// Must be called with sq.mu held.
func (sq *SessionQueue) startNext(ctx context.Context) {
	if len(sq.queue) == 0 {
		return
	}

	pending := sq.queue[0]
	sq.queue = sq.queue[1:]
	sq.active = true

	runCtx, cancel := context.WithCancel(ctx)
	sq.cancel = cancel

	lane := sq.laneMgr.Get(sq.lane)
	if lane == nil {
		lane = sq.laneMgr.Get("main")
	}

	if lane == nil {
		// No lane available — run directly
		go sq.executeRun(runCtx, pending)
		return
	}

	err := lane.Submit(ctx, func() {
		sq.executeRun(runCtx, pending)
	})
	if err != nil {
		pending.ResultCh <- RunOutcome{Err: err}
		close(pending.ResultCh)
		// caller already holds sq.mu — set fields directly
		sq.active = false
		sq.cancel = nil
	}
}

// executeRun runs the agent and then processes the next queued message.
func (sq *SessionQueue) executeRun(ctx context.Context, pending *PendingRequest) {
	result, err := sq.runFn(ctx, pending.Req)
	pending.ResultCh <- RunOutcome{Result: result, Err: err}
	close(pending.ResultCh)

	sq.mu.Lock()
	sq.active = false
	sq.cancel = nil

	if len(sq.queue) > 0 {
		// Use parentCtx (not the per-run ctx which may be cancelled in interrupt mode)
		sq.scheduleNext(sq.parentCtx)
	}
	sq.mu.Unlock()
}

// applyDropPolicy handles a full queue.
// Must be called with sq.mu held.
func (sq *SessionQueue) applyDropPolicy(incoming *PendingRequest) {
	switch sq.config.Drop {
	case DropOld:
		// Drop the oldest queued message
		if len(sq.queue) > 0 {
			old := sq.queue[0]
			old.ResultCh <- RunOutcome{Err: ErrQueueDropped}
			close(old.ResultCh)
			sq.queue = sq.queue[1:]
		}
		sq.queue = append(sq.queue, incoming)

	case DropNew:
		// Reject the incoming message
		incoming.ResultCh <- RunOutcome{Err: ErrQueueFull}
		close(incoming.ResultCh)

	default:
		// Default to drop old
		if len(sq.queue) > 0 {
			old := sq.queue[0]
			old.ResultCh <- RunOutcome{Err: ErrQueueDropped}
			close(old.ResultCh)
			sq.queue = sq.queue[1:]
		}
		sq.queue = append(sq.queue, incoming)
	}
}

// drainQueue cancels all pending requests with the given outcome.
// Must be called with sq.mu held.
func (sq *SessionQueue) drainQueue(outcome RunOutcome) {
	for _, p := range sq.queue {
		p.ResultCh <- outcome
		close(p.ResultCh)
	}
	sq.queue = nil
}

// IsActive returns whether a run is currently executing.
func (sq *SessionQueue) IsActive() bool {
	sq.mu.Lock()
	defer sq.mu.Unlock()
	return sq.active
}

// QueueLen returns the number of pending messages.
func (sq *SessionQueue) QueueLen() int {
	sq.mu.Lock()
	defer sq.mu.Unlock()
	return len(sq.queue)
}

// Scheduler is the top-level coordinator that manages lanes and session queues.
type Scheduler struct {
	lanes    *LaneManager
	sessions map[string]*SessionQueue
	config   QueueConfig
	runFn    RunFunc
	mu       sync.RWMutex
}

// NewScheduler creates a scheduler with the given lane and queue config.
func NewScheduler(laneConfigs []LaneConfig, queueCfg QueueConfig, runFn RunFunc) *Scheduler {
	if laneConfigs == nil {
		laneConfigs = DefaultLanes()
	}

	return &Scheduler{
		lanes:    NewLaneManager(laneConfigs),
		sessions: make(map[string]*SessionQueue),
		config:   queueCfg,
		runFn:    runFn,
	}
}

// Schedule submits a run request to the appropriate session queue and lane.
// Returns a channel that receives the result when the run completes.
func (s *Scheduler) Schedule(ctx context.Context, lane string, req agent.RunRequest) <-chan RunOutcome {
	sq := s.getOrCreateSession(req.SessionKey, lane)
	return sq.Enqueue(ctx, req)
}

// getOrCreateSession returns or creates a session queue for the given key.
func (s *Scheduler) getOrCreateSession(sessionKey, lane string) *SessionQueue {
	s.mu.RLock()
	sq, ok := s.sessions[sessionKey]
	s.mu.RUnlock()

	if ok {
		return sq
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if sq, ok := s.sessions[sessionKey]; ok {
		return sq
	}

	sq = NewSessionQueue(sessionKey, lane, s.config, s.lanes, s.runFn)
	s.sessions[sessionKey] = sq

	slog.Debug("session queue created", "session", sessionKey, "lane", lane)
	return sq
}

// Stop shuts down all lanes and clears session queues.
func (s *Scheduler) Stop() {
	s.lanes.StopAll()
}

// LaneStats returns utilization metrics for all lanes.
func (s *Scheduler) LaneStats() []LaneStats {
	return s.lanes.AllStats()
}

// Lanes returns the underlying lane manager (for direct access if needed).
func (s *Scheduler) Lanes() *LaneManager {
	return s.lanes
}
