package report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// ReportInterruptWindow is the durable per-worker urgent interruption limit.
const ReportInterruptWindow = 3 * time.Minute

// OrchestratorBusyRetry is how long a batch refused with ErrOrchestratorBusy
// waits before its next delivery attempt.
const OrchestratorBusyRetry = 30 * time.Second

// ErrReportDeliveryModeUnsupported reports a target without semantic delivery.
var ErrReportDeliveryModeUnsupported = errors.New("report delivery requires a semantic conversation")

// ErrOrchestratorBusy marks a Submit refused because the orchestrator cannot
// take the batch right now — its composer holds the operator's unsent draft —
// and should be retried later rather than immediately.
var ErrOrchestratorBusy = errors.New("orchestrator cannot take reports right now")

// DeliveryStore is the durable report scheduling and routing boundary.
type DeliveryStore interface {
	ListSessions(context.Context, domain.ProjectID) ([]domain.SessionRecord, error)
	GetSession(context.Context, domain.SessionID) (domain.SessionRecord, bool, error)
	ListPendingReportSchedule(context.Context, int64) ([]domain.ReportRecord, error)
	ClaimPendingReportBatch(context.Context, domain.ProjectID, string, time.Time) ([]domain.ReportRecord, error)
	AcknowledgeReportBatch(context.Context, domain.ProjectID, string, time.Time) (int, error)
	ReleaseReportBatch(context.Context, domain.ProjectID, string, time.Time, string) (int, error)
	DeferReportBatch(context.Context, domain.ProjectID, string, time.Time, string) (int, error)
	AcquireReportInterrupt(context.Context, domain.SessionID, time.Time, time.Duration) (bool, time.Time, error)
}

// Delivery submits accepted semantic context and interrupts active work.
type Delivery interface {
	Submit(context.Context, domain.SessionID, string, string) error
	Interrupt(context.Context, domain.SessionID) error
}

// Coordinator delivers durable report batches to active orchestrators.
type Coordinator struct {
	store      DeliveryStore
	delivery   Delivery
	now        func() time.Time
	newToken   func() string
	retryDelay time.Duration
	wake       chan struct{}
	mu         sync.Mutex
}

// CoordinatorDeps configures durable scheduling and delivery.
type CoordinatorDeps struct {
	Store      DeliveryStore
	Delivery   Delivery
	Now        func() time.Time
	NewToken   func() string
	RetryDelay time.Duration
}

// NewCoordinator constructs an idle report delivery coordinator.
func NewCoordinator(d CoordinatorDeps) *Coordinator {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.NewToken == nil {
		d.NewToken = uuid.NewString
	}
	if d.RetryDelay <= 0 {
		d.RetryDelay = 5 * time.Second
	}
	return &Coordinator{store: d.Store, delivery: d.Delivery, now: d.Now, newToken: d.NewToken, retryDelay: d.RetryDelay, wake: make(chan struct{}, 1)}
}

// Wake asks the coordinator to inspect durable scheduling state.
func (c *Coordinator) Wake() {
	if c == nil {
		return
	}
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Start runs delivery until ctx is cancelled and returns its completion signal.
func (c *Coordinator) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	go func() { defer close(done); c.run(ctx) }()
	return done
}

func (c *Coordinator) run(ctx context.Context) {
	for {
		delay, err := c.RunDue(ctx)
		if err != nil {
			delay = c.retryDelay
		}
		if delay < 0 {
			delay = time.Hour
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-c.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

// RunDue performs bounded due delivery and returns the delay until the next
// durable deadline. Tests call it directly with fake time.
func (c *Coordinator) RunDue(ctx context.Context) (time.Duration, error) {
	if c == nil || c.store == nil || c.delivery == nil {
		return -1, errors.New("report coordinator dependencies are required")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		pending, err := c.store.ListPendingReportSchedule(ctx, 1)
		if err != nil {
			return c.retryDelay, err
		}
		if len(pending) == 0 {
			return -1, nil
		}
		now := c.now().UTC()
		if pending[0].AvailableAt.After(now) {
			return pending[0].AvailableAt.Sub(now), nil
		}
		if err := c.deliverProject(ctx, pending[0].ProjectID); err != nil {
			return c.retryDelay, err
		}
	}
}

func (c *Coordinator) deliverProject(ctx context.Context, projectID domain.ProjectID) error {
	orchestrators, err := c.activeOrchestratorsByFolder(ctx, projectID)
	if err != nil {
		return err
	}
	if len(orchestrators) == 0 {
		return fmt.Errorf("project %s has no active orchestrator", projectID)
	}
	batch, err := c.claim(ctx, projectID)
	if err != nil || len(batch.Reports) == 0 {
		return err
	}
	// Resolve each report's target orchestrator (exact folder match, or main
	// orchestrator with empty workspace folder as fallback). Group preserves order.
	groups, missingFolder, err := c.groupBatchByTarget(ctx, batch, orchestrators)
	if err != nil {
		return c.release(ctx, batch, err)
	}
	if missingFolder {
		return c.retryLater(ctx, batch, fmt.Errorf("project %s has no active orchestrator", projectID))
	}
	for idx, group := range groups {
		target := group.target
		subBatch := group.batch
		idempotencyKey := subBatch.IdempotencyKey()
		if len(groups) > 1 {
			idempotencyKey = fmt.Sprintf("%s:%d:%s", idempotencyKey, idx, target.ID)
		}
		// Terminal orchestrators queue typed input during a turn, and a Ctrl-C
		// there cancels the running turn or a prompt that was just submitted, so
		// stuck reports are delivered to them like any other report.
		if subBatch.hasState(domain.ReportStuck) && domain.NormalizeSessionMode(target.Mode) == domain.SessionModeChat {
			interrupted := false
			var nextInterrupt time.Time
			for _, worker := range subBatch.stuckWorkers() {
				acquired, next, acquireErr := c.store.AcquireReportInterrupt(ctx, worker, c.now().UTC(), ReportInterruptWindow)
				if acquireErr != nil {
					return c.release(ctx, batch, acquireErr)
				}
				if acquired {
					interrupted = true
					if interruptErr := c.delivery.Interrupt(ctx, target.ID); interruptErr != nil {
						return c.release(ctx, batch, interruptErr)
					}
				} else if nextInterrupt.IsZero() || next.Before(nextInterrupt) {
					nextInterrupt = next
				}
			}
			if !interrupted && !subBatch.hasState(domain.ReportNeedsInput) && len(groups) == 1 {
				_, deferErr := c.store.DeferReportBatch(ctx, batch.ProjectID, batch.Token, nextInterrupt, "repeated stuck report coalesced inside interrupt window")
				return deferErr
			}
			if !interrupted && !subBatch.hasState(domain.ReportNeedsInput) {
				continue
			}
		}
		if err := c.delivery.Submit(ctx, target.ID, subBatch.Render(), idempotencyKey); err != nil {
			if errors.Is(err, ErrOrchestratorBusy) {
				// Released, the batch would stay the earliest due and every retry would
				// stop RunDue before any other project's reports. Back off instead.
				_, deferErr := c.store.DeferReportBatch(ctx, batch.ProjectID, batch.Token, c.now().UTC().Add(OrchestratorBusyRetry), err.Error())
				return deferErr
			}
			return c.retryLater(ctx, batch, err)
		}
	}
	_, err = c.store.AcknowledgeReportBatch(ctx, batch.ProjectID, batch.Token, c.now().UTC())
	return err
}

type targetedReportBatch struct {
	target domain.SessionRecord
	batch  PreparedBatch
}

func (c *Coordinator) activeOrchestratorsByFolder(ctx context.Context, projectID domain.ProjectID) (map[string]domain.SessionRecord, error) {
	recs, err := c.store.ListSessions(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byFolder := make(map[string]domain.SessionRecord)
	for _, rec := range recs {
		if rec.Kind != domain.KindOrchestrator || rec.IsTerminated || rec.Activity.State == domain.ActivityExited {
			continue
		}
		folder := rec.Metadata.WorkspaceFolder
		existing, ok := byFolder[folder]
		if !ok || rec.CreatedAt.After(existing.CreatedAt) {
			byFolder[folder] = rec
		}
	}
	return byFolder, nil
}

func resolveReportTargetOrchestrator(byFolder map[string]domain.SessionRecord, folder string) (domain.SessionRecord, bool) {
	if folder != "" {
		if orch, ok := byFolder[folder]; ok {
			return orch, true
		}
	}
	if main, ok := byFolder[""]; ok {
		return main, true
	}
	return domain.SessionRecord{}, false
}

func (c *Coordinator) sessionWorkspaceFolder(ctx context.Context, id domain.SessionID) (string, error) {
	sess, found, err := c.store.GetSession(ctx, id)
	if err != nil || !found {
		return "", err
	}
	return sess.Metadata.WorkspaceFolder, nil
}

func (c *Coordinator) groupBatchByTarget(
	ctx context.Context,
	batch PreparedBatch,
	byFolder map[string]domain.SessionRecord,
) ([]targetedReportBatch, bool, error) {
	order := make([]domain.SessionID, 0)
	byTarget := make(map[domain.SessionID]*targetedReportBatch)
	for _, r := range batch.Reports {
		folder, err := c.sessionWorkspaceFolder(ctx, r.SessionID)
		if err != nil {
			return nil, false, err
		}
		target, ok := resolveReportTargetOrchestrator(byFolder, folder)
		if !ok {
			return nil, true, nil
		}
		entry, exists := byTarget[target.ID]
		if !exists {
			entry = &targetedReportBatch{
				target: target,
				batch: PreparedBatch{
					ProjectID: batch.ProjectID,
					Token:     batch.Token,
					ID:        batch.ID,
				},
			}
			byTarget[target.ID] = entry
			order = append(order, target.ID)
		}
		entry.batch.Reports = append(entry.batch.Reports, r)
	}
	out := make([]targetedReportBatch, 0, len(order))
	for _, id := range order {
		out = append(out, *byTarget[id])
	}
	return out, false, nil
}

// PreparedBatch is a token-fenced durable batch awaiting acceptance.
type PreparedBatch struct {
	ProjectID domain.ProjectID
	Token     string
	ID        string
	Reports   []domain.ReportRecord
}

func (c *Coordinator) claim(ctx context.Context, projectID domain.ProjectID) (PreparedBatch, error) {
	token := c.newToken()
	reports, err := c.store.ClaimPendingReportBatch(ctx, projectID, token, c.now().UTC())
	for i := range reports {
		session, found, lookupErr := c.store.GetSession(ctx, reports[i].SessionID)
		if lookupErr == nil && found {
			reports[i].SessionDisplayName = strings.TrimSpace(session.DisplayName)
		}
	}
	batchID := ""
	if len(reports) > 0 {
		batchID = reports[0].DeliveryBatchID
	}
	return PreparedBatch{ProjectID: projectID, Token: token, ID: batchID, Reports: reports}, err
}

// PreparePiggyback claims pending reports for the active Chat orchestrator.
// The caller must acknowledge only after its user message is durably accepted.
func (c *Coordinator) PreparePiggyback(ctx context.Context, target domain.SessionID) (PreparedBatch, error) {
	if c == nil || c.store == nil {
		return PreparedBatch{}, nil
	}
	rec, ok, err := c.store.GetSession(ctx, target)
	if err != nil || !ok {
		return PreparedBatch{}, err
	}
	if rec.Kind != domain.KindOrchestrator || rec.IsTerminated || domain.NormalizeSessionMode(rec.Mode) != domain.SessionModeChat {
		return PreparedBatch{}, nil
	}
	batch, err := c.claim(ctx, rec.ProjectID)
	if err != nil || len(batch.Reports) == 0 {
		return batch, err
	}
	byFolder, err := c.activeOrchestratorsByFolder(ctx, rec.ProjectID)
	if err != nil {
		return PreparedBatch{}, c.release(ctx, batch, err)
	}
	for _, r := range batch.Reports {
		folder, folderErr := c.sessionWorkspaceFolder(ctx, r.SessionID)
		if folderErr != nil {
			return PreparedBatch{}, c.release(ctx, batch, folderErr)
		}
		resolved, found := resolveReportTargetOrchestrator(byFolder, folder)
		if !found || resolved.ID != target {
			// Batch includes a report routed to another folder/main orchestrator;
			// release without error so the coordinator delivers each to its target.
			_, releaseErr := c.store.ReleaseReportBatch(ctx, batch.ProjectID, batch.Token, c.now().UTC(), "report batch spans multiple orchestrators")
			return PreparedBatch{}, releaseErr
		}
	}
	return batch, nil
}

// AcceptPiggyback acknowledges reports after the combined user turn is accepted.
func (c *Coordinator) AcceptPiggyback(ctx context.Context, batch PreparedBatch) error {
	if len(batch.Reports) == 0 {
		return nil
	}
	_, err := c.store.AcknowledgeReportBatch(ctx, batch.ProjectID, batch.Token, c.now().UTC())
	return err
}

// ReleasePiggyback makes a batch retryable when the combined turn was refused.
func (c *Coordinator) ReleasePiggyback(ctx context.Context, batch PreparedBatch, cause error) error {
	if len(batch.Reports) == 0 {
		return nil
	}
	return c.release(ctx, batch, cause)
}

func (c *Coordinator) release(ctx context.Context, batch PreparedBatch, cause error) error {
	_, releaseErr := c.store.ReleaseReportBatch(ctx, batch.ProjectID, batch.Token, c.now().UTC(), cause.Error())
	if releaseErr != nil {
		return errors.Join(cause, releaseErr)
	}
	return cause
}

// retryLater queues a refused submission behind every report that is already
// due. Schedule selection takes the earliest pending report across projects,
// so an orchestrator that keeps refusing must not hold that place.
func (c *Coordinator) retryLater(ctx context.Context, batch PreparedBatch, cause error) error {
	_, deferErr := c.store.DeferReportBatch(ctx, batch.ProjectID, batch.Token, c.now().UTC().Add(c.retryDelay), cause.Error())
	if deferErr != nil {
		return errors.Join(cause, deferErr)
	}
	return cause
}

const (
	chatReportOpen  = "<ao-worker-reports>"
	chatReportClose = "</ao-worker-reports>"
)

// AppendToUserMessage places reports after the user's text in one semantic turn.
// The envelope lets AO's Chat UI hide daemon-authored context while preserving
// the exact prompt accepted by the provider for retry and recovery.
func (b PreparedBatch) AppendToUserMessage(message string) string {
	if len(b.Reports) == 0 {
		return message
	}
	return message + "\n\n" + chatReportOpen + "\n" + b.Render() + "\n" + chatReportClose
}

// IdempotencyKey returns the stable durable delivery identity.
func (b PreparedBatch) IdempotencyKey() string {
	if b.ID != "" {
		return b.ID
	}
	ids := make([]string, len(b.Reports))
	for i := range b.Reports {
		ids[i] = b.Reports[i].ID
	}
	sort.Strings(ids)
	sum := sha256.Sum256([]byte(strings.Join(ids, "\x00")))
	return "report-batch:" + hex.EncodeToString(sum[:16])
}

// Render formats compact attributed report context for an orchestrator.
func (b PreparedBatch) Render() string {
	var out strings.Builder
	out.WriteString("Reports since your previous turn:")
	for _, report := range coalesceReports(b.Reports) {
		state := string(report.State)
		if state == "" {
			state = "information"
		}
		fmt.Fprintf(&out, "\n\n[%s] %s", state, reportSessionLink(report))
		if report.RepeatCount > 1 {
			fmt.Fprintf(&out, " (repeated %d times)", report.RepeatCount)
		}
		text := report.Note
		if text == "" {
			text = report.Message
		}
		if text != "" {
			out.WriteString("\n")
			out.WriteString(text)
		}
		for _, output := range report.Outputs {
			fmt.Fprintf(&out, "\n%s: %s", output.Kind, output.Reference)
		}
	}
	return out.String()
}

func reportDisplayNameOrID(r domain.ReportRecord) string {
	if name := strings.TrimSpace(r.SessionDisplayName); name != "" {
		return name
	}
	return string(r.SessionID)
}

// reportSessionLink is the markdown attribution a standalone report shows.
// Piggybacked copies stay inside the hidden user-message envelope.
func reportSessionLink(r domain.ReportRecord) string {
	label := strings.NewReplacer("\\", "\\\\", "[", "\\[", "]", "\\]", "\n", " ").Replace(reportDisplayNameOrID(r))
	href := "ao://sessions/" + url.PathEscape(string(r.ProjectID)) + "/" + url.PathEscape(string(r.SessionID))
	return "[" + label + "](" + href + ")"
}

func (b PreparedBatch) hasState(state domain.ReportState) bool {
	for _, report := range b.Reports {
		if report.State == state {
			return true
		}
	}
	return false
}

func (b PreparedBatch) stuckWorkers() []domain.SessionID {
	seen := map[domain.SessionID]bool{}
	var out []domain.SessionID
	for _, report := range b.Reports {
		if report.State == domain.ReportStuck && !seen[report.SessionID] {
			seen[report.SessionID] = true
			out = append(out, report.SessionID)
		}
	}
	return out
}

func coalesceReports(in []domain.ReportRecord) []domain.ReportRecord {
	out := make([]domain.ReportRecord, 0, len(in))
	indexes := map[string]int{}
	for _, report := range in {
		key := reportKey(report)
		if report.State == domain.ReportStuck {
			if i, ok := indexes[key]; ok {
				out[i].RepeatCount += report.RepeatCount
				continue
			}
			indexes[key] = len(out)
		}
		out = append(out, report)
	}
	return out
}

func reportKey(report domain.ReportRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\x00%s\x00%s\x00%s", report.SessionID, report.State, report.Note, report.Message)
	for _, output := range report.Outputs {
		fmt.Fprintf(&b, "\x00%s\x00%s\x00%s", output.Kind, output.Reference, output.Label)
	}
	return b.String()
}
