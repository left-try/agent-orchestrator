package backlog

import "github.com/aoagents/agent-orchestrator/backend/internal/domain"

// CreateInput holds parameters for creating a backlog item from a discussion.
type CreateInput struct {
	ProjectID             domain.ProjectID
	OrchestratorSessionID domain.SessionID
	Title                 string
	Summary               string
	Prompt                string
}

// UpdateInput holds editable fields for updating a backlog item.
type UpdateInput struct {
	Title   string
	Summary string
	Prompt  string
	Status  domain.BacklogItemStatus
}

// ImplementInput configures spawning a worker session from a backlog item.
type ImplementInput struct {
	Harness       domain.AgentHarness
	Model         string
	InterfaceMode domain.SessionMode
	Async         bool
}

// ImplementResult contains the updated backlog item and spawned worker session.
type ImplementResult struct {
	Item    domain.BacklogItem
	Session domain.Session
}
