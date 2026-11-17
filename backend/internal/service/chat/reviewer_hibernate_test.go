package chat_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	chatsvc "github.com/aoagents/agent-orchestrator/backend/internal/service/chat"
	"github.com/aoagents/agent-orchestrator/backend/internal/storage/sqlite/store"
)

func TestIdleReviewerHibernatesAndResumesSameConversation(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	now := time.Now().UTC().Add(time.Hour)
	var clock atomic.Int64
	clock.Store(now.UnixNano())
	if err := st.UpsertReview(ctx, domain.Review{ID: "sleep-review", SessionID: testSession, ProjectID: testProject, Harness: domain.ReviewerCodex, InterfaceMode: domain.ReviewerInterfaceChat, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	first := &hibernationConversation{fakeConversation: newFakeConversation()}
	second := newFakeConversation()
	second.turnSeq = 1
	driver := &sequenceDriver{conversations: []ports.ChatConversation{first, second}}
	var ids atomic.Int32
	svc := chatsvc.New(chatsvc.Options{Store: st, Sessions: st, Reader: fullSnapshotReader(st), Drivers: fakeRegistry{driver: driver}, Now: func() time.Time { return time.Unix(0, clock.Load()).UTC() }, NewID: func() string { return fmt.Sprintf("review-sleep-%d", ids.Add(1)) }})
	t.Cleanup(func() { svc.StopAll(context.Background()) })
	owner := domain.ReviewConversationOwner("sleep-review")
	if _, err := svc.Start(ctx, chatsvc.StartConfig{Owner: owner, SessionID: testSession, ProjectID: testProject, Harness: domain.HarnessCodex, WorkspacePath: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SendForOwner(ctx, owner, ports.ChatUserMessage{Text: "review this", ClientMessageID: "initial"}); err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(chatsvc.IdleBeforeHibernate + time.Minute))
	if slept, err := svc.HibernateIdleReviewer(ctx, owner.ID); err != nil || slept {
		t.Fatalf("active reviewer hibernated: %v, %v", slept, err)
	}
	first.emit(ports.ChatEvent{Kind: ports.ChatEventTurnStarted, ProviderTurnID: "provider-turn-1"}, ports.ChatEvent{Kind: ports.ChatEventTurnCompleted, ProviderTurnID: "provider-turn-1", TurnState: domain.TurnStateCompleted})
	conversation, err := st.ConversationForReview(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	awaitStoreSnapshot(t, st, conversation.ID, func(s store.ConversationSnapshot) bool {
		return len(s.Turns) == 1 && s.Turns[0].State == domain.TurnStateCompleted
	})
	clock.Add(int64(chatsvc.IdleBeforeHibernate + time.Minute))
	if err := st.InsertReviewRun(ctx, domain.ReviewRun{ID: "active-run", ReviewID: owner.ID, SessionID: testSession, Harness: domain.ReviewerCodex, PRURL: "https://example.test/pr/1", TargetSHA: "head", Status: domain.ReviewRunRunning, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if slept, err := svc.HibernateIdleReviewer(ctx, owner.ID); err != nil || slept {
		t.Fatalf("reviewer with running review hibernated: %v, %v", slept, err)
	}
	if _, err := st.CancelRunningReviewRunsBySession(ctx, testSession, "test complete"); err != nil {
		t.Fatal(err)
	}
	first.backgroundRunning = true
	if slept, err := svc.HibernateIdleReviewer(ctx, owner.ID); err != nil || slept {
		t.Fatalf("reviewer with background work hibernated: %v, %v", slept, err)
	}
	first.backgroundRunning = false
	first.started = make(chan struct{})
	release := make(chan struct{})
	first.release = release
	type sleepResult struct {
		slept bool
		err   error
	}
	stopped := make(chan sleepResult, 1)
	go func() { slept, err := svc.HibernateIdleReviewer(ctx, owner.ID); stopped <- sleepResult{slept, err} }()
	select {
	case <-first.started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider shutdown did not start")
	}
	snapshot, err := svc.SnapshotForReview(ctx, owner.ID)
	if err != nil || snapshot.Controller != ports.ChatControllerHibernated {
		close(release)
		t.Fatalf("sleeping snapshot = %v, %v", snapshot.Controller, err)
	}
	sent := make(chan error, 1)
	go func() {
		_, err := svc.SendForOwner(ctx, owner, ports.ChatUserMessage{Text: "follow up", ClientMessageID: "follow-up"})
		sent <- err
	}()
	close(release)
	result := <-stopped
	if result.err != nil || !result.slept {
		t.Fatalf("hibernate = %v, %v", result.slept, result.err)
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if first.calls.Load() != 1 {
		t.Fatal("provider not stopped")
	}
	if len(second.sentTexts()) != 1 {
		t.Fatal("follow-up was not delivered to resumed provider")
	}
	resumed, err := st.ConversationForReview(ctx, owner.ID)
	if err != nil || resumed.ID != conversation.ID {
		t.Fatalf("review conversation changed: %q -> %q (%v)", conversation.ID, resumed.ID, err)
	}
}
