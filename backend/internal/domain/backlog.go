package domain

import (
	"errors"
	"strings"
	"time"
)

// BacklogItemID identifies a backlog item stored from an unimplemented discussion.
type BacklogItemID string

// BacklogItemStatus tracks whether a backlog item is still open or has been started as a worker task.
type BacklogItemStatus string

const (
	// BacklogStatusOpen is an unimplemented discussion saved to the backlog.
	BacklogStatusOpen BacklogItemStatus = "open"
	// BacklogStatusImplemented is a backlog item that has been turned into a worker session.
	BacklogStatusImplemented BacklogItemStatus = "implemented"
)

// Valid reports whether s is a supported backlog item status.
func (s BacklogItemStatus) Valid() bool {
	return s == BacklogStatusOpen || s == BacklogStatusImplemented
}

// Backlog field size limits.
const (
	MaxBacklogTitleLength   = 160
	MaxBacklogSummaryLength = 4096
	MaxBacklogPromptLength  = 16 << 10
)

var (
	ErrInvalidBacklogTitle   = errors.New("invalid backlog title")
	ErrInvalidBacklogSummary = errors.New("invalid backlog summary")
	ErrInvalidBacklogPrompt  = errors.New("invalid backlog prompt")
	ErrInvalidBacklogStatus  = errors.New("invalid backlog status")
	ErrBacklogNotFound       = errors.New("backlog item not found")
)

// BacklogItem represents an unimplemented discussion saved for later work on the Kanban board.
type BacklogItem struct {
	ID                    BacklogItemID
	ProjectID             ProjectID
	OrchestratorSessionID string
	Title                 string
	Summary               string
	Prompt                string
	Status                BacklogItemStatus
	SpawnedSessionID      string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Validate checks required fields and bounds on a backlog item.
func (b BacklogItem) Validate() error {
	if strings.TrimSpace(b.Title) == "" || len(b.Title) > MaxBacklogTitleLength {
		return ErrInvalidBacklogTitle
	}
	if len(b.Summary) > MaxBacklogSummaryLength {
		return ErrInvalidBacklogSummary
	}
	if len(b.Prompt) > MaxBacklogPromptLength {
		return ErrInvalidBacklogPrompt
	}
	if !b.Status.Valid() {
		return ErrInvalidBacklogStatus
	}
	return nil
}
