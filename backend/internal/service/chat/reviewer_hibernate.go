package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

type reviewerRunningStore interface {
	ListRunningReviewRunsBySession(context.Context, domain.SessionID) ([]domain.ReviewRun, error)
}

// HibernateIdleReviewers releases idle review providers without affecting worker chats.
func (s *Service) HibernateIdleReviewers(ctx context.Context) error {
	s.mu.RLock()
	var ids []string
	for owner := range s.ownerControllers {
		if owner.Kind == domain.ConversationOwnerReview {
			ids = append(ids, owner.ID)
		}
	}
	s.mu.RUnlock()
	var errs []error
	for _, id := range ids {
		if _, err := s.HibernateIdleReviewer(ctx, id); err != nil {
			errs = append(errs, fmt.Errorf("reviewer %s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// HibernateIdleReviewer only stops a settled, resumable reviewer with no live work.
func (s *Service) HibernateIdleReviewer(ctx context.Context, id string) (bool, error) {
	owner := domain.ReviewConversationOwner(id)
	gate := s.controllerGate(owner)
	if !gate.tryLock() {
		return false, nil
	}
	defer gate.unlock()
	store, ok := s.store.(reviewerConversationStore)
	if !ok {
		return false, nil
	}
	runs, ok := s.store.(reviewerRunningStore)
	if !ok {
		return false, nil
	}
	turns, ok := s.sessions.(hibernationTurnReader)
	if !ok {
		return false, nil
	}
	review, found, err := store.GetReviewByID(ctx, id)
	if err != nil || !found {
		return false, err
	}
	if review.IsArchived || review.InterfaceMode != domain.ReviewerInterfaceChat || review.UpdatedAt.Add(IdleBeforeHibernate).After(s.now()) {
		return false, nil
	}
	active, err := runs.ListRunningReviewRunsBySession(ctx, review.SessionID)
	if err != nil {
		return false, err
	}
	for _, run := range active {
		if run.ReviewID == id || (run.ReviewID == "" && (run.Harness == "" || run.Harness == review.Harness)) {
			return false, nil
		}
	}
	controller, err := s.ControllerForOwner(owner)
	if err != nil || controller.State() != ports.ChatControllerReady || !controller.Capabilities().Has(ports.ChatCapabilityResume) {
		return false, nil
	}
	hibernator, ok := controller.conv.(ports.ChatProviderHibernator)
	if !ok || controller.ProviderConversationID() == "" {
		return false, nil
	}
	release := controller.beginOperation()
	ready, err := hibernator.CanHibernate(ctx)
	release()
	if err != nil || !ready {
		return false, err
	}
	if !controller.sendMu.TryLock() {
		return false, nil
	}
	locked := true
	defer func() {
		if locked {
			controller.sendMu.Unlock()
		}
	}()
	controller.mu.Lock()
	busy := controller.state != ports.ChatControllerReady || controller.handoff != controllerHandoffNone || controller.pendingTurnID != "" || controller.dispatchingTurnID != "" || controller.compactionPending || controller.operations != 0
	controller.mu.Unlock()
	if busy {
		return false, nil
	}
	conversation, err := store.ConversationForReview(ctx, id)
	if err != nil {
		return false, err
	}
	if conversation.ID != controller.conversation.ID || conversation.UpdatedAt.Add(IdleBeforeHibernate).After(s.now()) {
		return false, nil
	}
	if _, err := s.store.NextQueuedTurn(ctx, controller.conversation.ID); err == nil {
		return false, nil
	} else if !errors.Is(err, domain.ErrNoQueuedTurn) {
		return false, err
	}
	if running, err := s.store.ListVisibleRunningTurnProviderIDs(ctx, controller.conversation.ID); err != nil {
		return false, err
	} else if len(running) != 0 {
		return false, nil
	}
	if pending, err := s.store.HasPendingConversationInteractions(ctx, controller.conversation.ID); err != nil {
		return false, err
	} else if pending {
		return false, nil
	}
	settled, err := turns.LatestVisibleUserTurnSettled(ctx, controller.conversation.ID, controller.sessionID)
	if err != nil || !settled {
		return false, err
	}
	s.mu.Lock()
	cfg, exists := s.startConfigs[owner]
	if !exists || s.ownerControllers[owner] != controller {
		s.mu.Unlock()
		return false, nil
	}
	cfg.ProviderConversationID = controller.ProviderConversationID()
	s.startConfigs[owner] = cfg
	s.sleepingReviews[owner] = true
	s.mu.Unlock()
	controller.mu.Lock()
	controller.handoff = controllerHandoffHibernate
	controller.suppressStoppedActivity = true
	controller.mu.Unlock()
	controller.sendMu.Unlock()
	locked = false
	stopErr := hibernator.Hibernate()
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if stopErr == nil {
		select {
		case <-controller.stopped:
			return true, nil
		case <-finishCtx.Done():
			stopErr = finishCtx.Err()
		}
	}
	if s.stopProviderHost == nil {
		return false, fmt.Errorf("stop reviewer provider: %w", stopErr)
	}
	recoveryCtx, recoveryCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer recoveryCancel()
	if err := s.stopProviderHost(recoveryCtx, domain.SessionID("review-"+id)); err != nil {
		return false, fmt.Errorf("confirm reviewer shutdown: %w", errors.Join(stopErr, err))
	}
	select {
	case <-controller.stopped:
		return true, nil
	case <-recoveryCtx.Done():
		return false, fmt.Errorf("wait for reviewer shutdown: %w", recoveryCtx.Err())
	}
}

// wakeReviewerLocked is serialized with shutdown, archive and other sends.
func (s *Service) wakeReviewerLocked(ctx context.Context, owner domain.ConversationOwner) error {
	s.mu.RLock()
	asleep, cfg, previous := s.sleepingReviews[owner], s.startConfigs[owner], s.ownerControllers[owner]
	s.mu.RUnlock()
	if !asleep {
		return nil
	}
	if previous != nil {
		waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		select {
		case <-previous.stopped:
		case <-waitCtx.Done():
			return fmt.Errorf("wait for reviewer shutdown: %w", waitCtx.Err())
		}
	}
	_, err := s.startLocked(ctx, cfg)
	return err
}
