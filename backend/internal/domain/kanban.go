package domain

import (
	"errors"
	"strings"
	"unicode"

	"github.com/aoagents/agent-orchestrator/backend/pkg/contract"
)

// KanbanColumn is the derived delivery-lifecycle placement of a session:
// building, the AO-driven validating loop, the review-feedback loop
// (needs_review), ready, done, or archive.
type KanbanColumn = contract.KanbanColumn

// Kanban columns.
const (
	KanbanBacklog     KanbanColumn = "backlog"
	KanbanBuilding    = contract.KanbanBuilding
	KanbanValidating  = contract.KanbanValidating
	KanbanNeedsReview = contract.KanbanNeedsReview
	KanbanReady       = contract.KanbanReady
	KanbanDone        = contract.KanbanDone
	KanbanArchive     = contract.KanbanArchive
)

// DisplayStatus is the phrase shown inside a session's Kanban column: what is
// happening right now at the stage the column already placed it in.
type DisplayStatus = contract.DisplayStatus

// KanbanPresentation is a session's derived board presentation: its column plus
// the display status derived inside that column.
type KanbanPresentation = contract.KanbanPresentation

// MaxKanbanColumnLabelLength bounds a user-customized board column heading.
const MaxKanbanColumnLabelLength = 40

// ErrInvalidKanbanSettings reports malformed Kanban board customization preferences.
var ErrInvalidKanbanSettings = errors.New("invalid kanban settings")

// ConfigurableKanbanColumns lists the board lanes a user can enable, disable,
// rename, and reorder. Archive remains a modal surface rather than a lane.
var ConfigurableKanbanColumns = []KanbanColumn{
	KanbanBacklog,
	KanbanBuilding,
	KanbanValidating,
	KanbanNeedsReview,
	KanbanReady,
	KanbanDone,
}

// KanbanColumnPreference holds per-column board visibility, order, and optional label override.
type KanbanColumnPreference struct {
	ID      string `json:"id" enum:"backlog,building,validating,needs_review,ready,done"`
	Enabled bool   `json:"enabled"`
	Label   string `json:"label,omitempty" description:"Optional custom display label for this column; empty uses the localized default."`
}

// KanbanSettings holds the user's Kanban board customization preferences.
type KanbanSettings struct {
	MoveMergedToDone bool                     `json:"moveMergedToDone" description:"When true (default), sessions whose PRs are merged are placed in the Done column instead of Ready."`
	Columns          []KanbanColumnPreference `json:"columns"`
}

// DefaultKanbanSettings returns the default board layout: merged PRs move to
// Done by default, and Backlog is optional (off by default).
func DefaultKanbanSettings() KanbanSettings {
	return KanbanSettings{
		MoveMergedToDone: true,
		Columns: []KanbanColumnPreference{
			{ID: string(KanbanBacklog), Enabled: false},
			{ID: string(KanbanBuilding), Enabled: true},
			{ID: string(KanbanValidating), Enabled: true},
			{ID: string(KanbanNeedsReview), Enabled: true},
			{ID: string(KanbanReady), Enabled: true},
			{ID: string(KanbanDone), Enabled: true},
		},
	}
}

func isConfigurableKanbanColumn(id string) bool {
	for _, c := range ConfigurableKanbanColumns {
		if string(c) == id {
			return true
		}
	}
	return false
}

func defaultColumnEnabled(id string) bool {
	return id != string(KanbanBacklog)
}

// NormalizeKanbanSettings deduplicates columns, strips unknown IDs, trims labels,
// and appends any missing configurable columns with their default enabled state.
func NormalizeKanbanSettings(s KanbanSettings) KanbanSettings {
	seen := make(map[string]struct{}, len(ConfigurableKanbanColumns))
	cols := make([]KanbanColumnPreference, 0, len(ConfigurableKanbanColumns))
	for _, col := range s.Columns {
		id := strings.TrimSpace(col.ID)
		if !isConfigurableKanbanColumn(id) {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		cols = append(cols, KanbanColumnPreference{
			ID:      id,
			Enabled: col.Enabled,
			Label:   strings.TrimSpace(col.Label),
		})
	}
	for _, c := range ConfigurableKanbanColumns {
		id := string(c)
		if _, ok := seen[id]; !ok {
			cols = append(cols, KanbanColumnPreference{
				ID:      id,
				Enabled: defaultColumnEnabled(id),
			})
		}
	}
	return KanbanSettings{
		MoveMergedToDone: s.MoveMergedToDone,
		Columns:          cols,
	}
}

// ValidateKanbanSettings checks an incoming Kanban customization payload.
func ValidateKanbanSettings(s KanbanSettings) (KanbanSettings, error) {
	if len(s.Columns) == 0 {
		return KanbanSettings{}, errors.New("columns must not be empty")
	}
	seen := make(map[string]struct{}, len(s.Columns))
	enabledCount := 0
	cols := make([]KanbanColumnPreference, 0, len(ConfigurableKanbanColumns))
	for _, col := range s.Columns {
		id := strings.TrimSpace(col.ID)
		if !isConfigurableKanbanColumn(id) {
			return KanbanSettings{}, errors.New("unknown kanban column id: " + col.ID)
		}
		if _, dup := seen[id]; dup {
			return KanbanSettings{}, errors.New("duplicate kanban column id: " + id)
		}
		label := strings.TrimSpace(col.Label)
		if len(label) > MaxKanbanColumnLabelLength || strings.ContainsFunc(label, unicode.IsControl) {
			return KanbanSettings{}, errors.New("invalid kanban column label")
		}
		seen[id] = struct{}{}
		if col.Enabled {
			enabledCount++
		}
		cols = append(cols, KanbanColumnPreference{
			ID:      id,
			Enabled: col.Enabled,
			Label:   label,
		})
	}
	for _, c := range ConfigurableKanbanColumns {
		id := string(c)
		if _, ok := seen[id]; !ok {
			enabled := defaultColumnEnabled(id)
			if enabled {
				enabledCount++
			}
			cols = append(cols, KanbanColumnPreference{
				ID:      id,
				Enabled: enabled,
			})
		}
	}
	if enabledCount == 0 {
		return KanbanSettings{}, errors.New("at least one kanban column must remain enabled")
	}
	return KanbanSettings{
		MoveMergedToDone: s.MoveMergedToDone,
		Columns:          cols,
	}, nil
}
