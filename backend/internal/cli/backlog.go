package cli

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type backlogItemDTO struct {
	ID                    string `json:"id"`
	ProjectID             string `json:"projectId"`
	OrchestratorSessionID string `json:"orchestratorSessionId,omitempty"`
	Title                 string `json:"title"`
	Summary               string `json:"summary,omitempty"`
	Prompt                string `json:"prompt,omitempty"`
	Status                string `json:"status"`
	SpawnedSessionID      string `json:"spawnedSessionId,omitempty"`
}

type backlogItemEnvelopeDTO struct {
	Item backlogItemDTO `json:"item"`
}

type backlogListDTO struct {
	Items []backlogItemDTO `json:"items"`
}

type backlogCreateDTO struct {
	OrchestratorSessionID string `json:"orchestratorSessionId,omitempty"`
	Title                 string `json:"title"`
	Summary               string `json:"summary,omitempty"`
	Prompt                string `json:"prompt,omitempty"`
}

type backlogUpdateDTO struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
	Status  string `json:"status,omitempty"`
}

type backlogImplementDTO struct {
	Harness       string `json:"harness,omitempty"`
	Model         string `json:"model,omitempty"`
	InterfaceMode string `json:"interfaceMode,omitempty"`
}

type backlogImplementResponseDTO struct {
	Item      backlogItemDTO `json:"item"`
	SessionID string         `json:"sessionId"`
}

func newBacklogCommand(ctx *commandContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backlog",
		Short: "Manage Kanban backlog items saved from discussions",
		Long:  "Save unimplemented discussions to the Kanban backlog, list or update them, or start a worker task from an item.",
	}
	cmd.AddCommand(
		newBacklogAddCommand(ctx),
		newBacklogListCommand(ctx),
		newBacklogUpdateCommand(ctx),
		newBacklogImplementCommand(ctx),
		newBacklogDeleteCommand(ctx),
	)
	return cmd
}

func newBacklogAddCommand(ctx *commandContext) *cobra.Command {
	var project, orchestratorSession, title, summary, prompt string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Save an unimplemented discussion to the Kanban backlog",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(title) == "" {
				return usageError{errors.New("--title is required")}
			}
			resolved, err := ctx.resolveSpawnProject(cmd.Context(), project)
			if err != nil {
				return err
			}
			sessionID := strings.TrimSpace(orchestratorSession)
			if sessionID == "" {
				sessionID = strings.TrimSpace(os.Getenv("AO_SESSION_ID"))
			}
			body := backlogCreateDTO{
				OrchestratorSessionID: sessionID,
				Title:                 title,
				Summary:               summary,
				Prompt:                prompt,
			}
			var response backlogItemEnvelopeDTO
			if err := ctx.postJSON(cmd.Context(), "projects/"+url.PathEscape(resolved.ID)+"/backlog", body, &response); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), response)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "saved backlog item %s (%s) in project %s\n", response.Item.ID, response.Item.Title, response.Item.ProjectID)
			return err
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project id (default: AO_PROJECT_ID, current session, or current registered repo)")
	cmd.Flags().StringVar(&orchestratorSession, "orchestrator-session", "", "Orchestrator session id where the discussion occurred (default: AO_SESSION_ID)")
	cmd.Flags().StringVar(&title, "title", "", "Short title for the backlog card")
	cmd.Flags().StringVar(&summary, "summary", "", "Summary of the unimplemented discussion")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Optional implementation prompt for spawning a worker later")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")
	return cmd
}

func newBacklogListCommand(ctx *commandContext) *cobra.Command {
	var project string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List backlog items for a project",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := ctx.resolveSpawnProject(cmd.Context(), project)
			if err != nil {
				return err
			}
			var response backlogListDTO
			if err := ctx.getJSON(cmd.Context(), "projects/"+url.PathEscape(resolved.ID)+"/backlog", &response); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), response)
			}
			writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(writer, "ID\tSTATUS\tTITLE\tSESSION")
			for _, item := range response.Items {
				_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", item.ID, item.Status, item.Title, item.SpawnedSessionID)
			}
			return writer.Flush()
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "Project id (default: AO_PROJECT_ID, current session, or current registered repo)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")
	return cmd
}

func newBacklogUpdateCommand(ctx *commandContext) *cobra.Command {
	var title, summary, prompt, status string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "update <backlog-id>",
		Short: "Update a backlog item",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := backlogUpdateDTO{
				Title:   title,
				Summary: summary,
				Prompt:  prompt,
				Status:  status,
			}
			var response backlogItemEnvelopeDTO
			if err := ctx.patchJSON(cmd.Context(), "backlog/"+url.PathEscape(args[0]), body, &response); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), response)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "updated backlog item %s (%s)\n", response.Item.ID, response.Item.Title)
			return err
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Updated title")
	cmd.Flags().StringVar(&summary, "summary", "", "Updated summary")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Updated implementation prompt")
	cmd.Flags().StringVar(&status, "status", "", "Status (open or implemented)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")
	return cmd
}

func newBacklogImplementCommand(ctx *commandContext) *cobra.Command {
	var harness, model, mode string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "implement <backlog-id>",
		Short: "Start a worker session from a backlog item",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := backlogImplementDTO{
				Harness:       harness,
				Model:         model,
				InterfaceMode: mode,
			}
			var response backlogImplementResponseDTO
			if err := ctx.postJSON(cmd.Context(), "backlog/"+url.PathEscape(args[0])+"/implement", body, &response); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), response)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "started worker %s from backlog item %s\n", response.SessionID, response.Item.ID)
			return err
		},
	}
	cmd.Flags().StringVar(&harness, "harness", "", "Agent harness override")
	cmd.Flags().StringVar(&model, "model", "", "Model override")
	cmd.Flags().StringVar(&mode, "mode", "", "Session interface mode (chat or tui)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print JSON")
	return cmd
}

func newBacklogDeleteCommand(ctx *commandContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <backlog-id>",
		Aliases: []string{"rm"},
		Short:   "Remove a backlog item",
		Args:    usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ctx.deleteJSON(cmd.Context(), "backlog/"+url.PathEscape(args[0]), nil); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "deleted backlog item %s\n", args[0])
			return err
		},
	}
	return cmd
}
