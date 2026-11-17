// Package agyacp binds Google Antigravity to AO's reusable ACP Chat transport.
//
// The agy CLI has a built-in ACP server, `agy acp`, which steers a running turn
// when another prompt arrives. AO launches it from agy 1.5.4, the first release
// whose `agy acp` does not hang on a second approval. Older CLIs have no ACP
// mode, or that bug, so AO falls back to
// Google's separate `agy_acp_server` from the ACP registry (`antigravity-acp`),
// found beside the agy CLI or on PATH. Like Autohand's separately distributed
// `autohand-acp`, AO never downloads or bundles either. Login, models, MCP
// configuration, and conversations stay the user's own under GEMINI_HOME
// (default ~/.gemini); both servers keep Chat conversations in the same
// antigravity-acp folder.
package agyacp

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/agent/agy"
	acpdriver "github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/acp"
	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// Plugin is the existing Agy agent-plugin surface the binding reuses: the agy
// CLI's location, beside which the ACP server is usually installed, and the
// canonical auth probe. The server itself is resolved separately because it is
// not the agy CLI.
type Plugin interface {
	ResolveBinary(context.Context) (string, error)
	AuthStatus(context.Context) (ports.AgentAuthStatus, error)
}

// Antigravity's ACP mode ids, as advertised by agy_acp_server 1.3.0.
const (
	modeDefault  = "default"
	modeAutoEdit = "auto_edit"
	modeYolo     = "yolo"
)

// `agy acp`'s mode ids. Its native Auto mode safely approves workspace edits
// and sandboxed commands while retaining review for commands outside the
// sandbox (Agy 1.5.4 changelog).
const (
	cliModeAcceptEdits = "accept-edits"
	cliModeAuto        = "auto"
)

// New constructs the Antigravity ACP Chat driver.
func New(plugin Plugin, log *slog.Logger) ports.ChatDriver {
	var agyCLI func(context.Context) (string, error)
	if plugin != nil {
		agyCLI = plugin.ResolveBinary
	}
	return acpdriver.New(buildConfig(plugin, newResolver(agyCLI), log), log)
}

func buildConfig(plugin Plugin, resolve resolver, log *slog.Logger) acpdriver.Config {
	return acpdriver.Config{
		Harness: domain.HarnessAgy,
		Capabilities: ports.ChatCapabilities{
			ports.ChatCapabilityStreaming: true,
			ports.ChatCapabilityTools:     true,
			// Antigravity raises session/request_permission for every gated tool.
			ports.ChatCapabilityApprovals: true,
			ports.ChatCapabilityInterrupt: true,
			ports.ChatCapabilityResume:    true,
		},
		Probe: func(ctx context.Context) error {
			if _, err := resolve.target(ctx); err != nil {
				return fmt.Errorf("%w: %w", ports.ErrChatDriverUnavailable, err)
			}
			if plugin != nil {
				status, err := plugin.AuthStatus(ctx)
				if err == nil && status == ports.AgentAuthStatusUnauthorized {
					return ports.ErrChatAuthRequired
				}
				if err != nil && log != nil {
					// The ACP handshake stays authoritative for an inconclusive probe.
					log.Debug("agy auth probe inconclusive; continuing", "error", err)
				}
			}
			return nil
		},
		Launch: func(ctx context.Context, cfg acpdriver.LaunchConfig) (acpdriver.Launch, error) {
			target, err := resolve.target(ctx)
			if err != nil {
				return acpdriver.Launch{}, fmt.Errorf("%w: %w", ports.ErrChatDriverUnavailable, err)
			}
			env := make(map[string]string, len(cfg.Env)+2)
			for key, value := range cfg.Env {
				env[key] = value
			}
			if env[disableWorkspaceTrustEnvVar] == "" {
				env[disableWorkspaceTrustEnvVar] = "1"
			}
			if target.cli {
				return acpdriver.Launch{Command: target.path, Args: cliLaunchArgs(env, resolve.getenv), Env: env}, nil
			}
			if env[harnessPathEnvVar] == "" {
				if harness := resolve.harnessPath(target.path); harness != "" {
					env[harnessPathEnvVar] = harness
				}
			}
			return acpdriver.Launch{Command: target.path, Args: launchArgs(runtime.GOOS), Env: env}, nil
		},
		ValidateInitialize:  validateInitialize,
		AfterInitialize:     afterInitialize(newAuthBootstrap(agy.GeminiHome)),
		SessionMode:         sessionMode,
		SessionModeForAgent: sessionModeForAgent,
		SessionOptions:      sessionOptions,
		SessionPreamble:     sessionPreamble,
		// Native Agy Auto decides which actions are safe; any request it still
		// raises remains with the user. AO never selects allow_always, which
		// could outlive a switch to a stricter approval mode.
		PermissionPolicy: permissionPolicy,
	}
}

// cliLaunchArgs starts `agy acp` with its Chat data in antigravity-acp, where
// the separate server keeps it, so AO's history checks and the Chat/terminal
// handoff find it, and a Chat started on the separate server resumes. `agy acp`
// ignores the GEMINI_HOME variable and stores under its --gemini-dir, so a
// GEMINI_HOME set for the session or inherited by the daemon is passed as that
// flag, the same precedence agy.GeminiHome uses.
func cliLaunchArgs(env map[string]string, getenv func(string) string) []string {
	args := []string{"acp", "--app-data-dir", agy.ACPStorageDir}
	home := strings.TrimSpace(env["GEMINI_HOME"])
	if home == "" {
		home = strings.TrimSpace(getenv("GEMINI_HOME"))
	}
	if home != "" {
		args = append(args, "--gemini-dir", home)
	}
	return args
}

// afterInitialize runs the separate server's sign-in bootstrap. `agy acp`
// reuses the CLI's own login, and its authenticate method could start a
// browser sign-in, so AO leaves it alone.
func afterInitialize(bootstrap authBootstrap) func(context.Context, *acpsdk.ClientSideConnection, acpsdk.InitializeResponse, acpdriver.LaunchConfig) error {
	return func(ctx context.Context, conn *acpsdk.ClientSideConnection, init acpsdk.InitializeResponse, cfg acpdriver.LaunchConfig) error {
		if isCLIServer(init) {
			return nil
		}
		return bootstrap.run(ctx, conn, init, cfg)
	}
}

// sessionModeForAgent maps AO's approvals onto `agy acp`'s modes. Its native
// Auto mode preserves Agy's sandbox and review behavior. The separate ACP
// server has no Auto mode, so AO Auto falls back to auto_edit and leaves
// command permission requests for the user. The CLI has no ACP yolo mode, so
// bypass uses accept-edits plus one-time answers to its permission requests.
func sessionModeForAgent(init acpsdk.InitializeResponse) func(ports.PermissionMode) string {
	if !isCLIServer(init) {
		return nil
	}
	return func(permission ports.PermissionMode) string {
		switch ports.NormalizePermissionMode(permission) {
		case ports.PermissionModeDefault:
			return modeDefault
		case ports.PermissionModeAcceptEdits:
			return cliModeAcceptEdits
		case ports.PermissionModeAuto:
			return cliModeAuto
		case ports.PermissionModeBypassPermissions:
			return cliModeAcceptEdits
		default:
			return ""
		}
	}
}

// permissionPolicy answers any residual request only for explicit bypass.
// Native `agy acp` Auto owns its safe-action review and must be allowed to ask
// about anything it considers risky. The separate server has no Auto mode, so
// AO Auto maps to auto_edit and leaves command requests to the user.
func permissionPolicy(permission ports.PermissionMode, request acpsdk.RequestPermissionRequest) (acpsdk.PermissionOptionId, bool) {
	if ports.NormalizePermissionMode(permission) != ports.PermissionModeBypassPermissions {
		return "", false
	}
	for _, option := range request.Options {
		if option.Kind == acpsdk.PermissionOptionKindAllowOnce {
			return option.OptionId, true
		}
	}
	return "", false
}

// launchArgs mirrors the registry entry's per-platform arguments. The Linux
// build requires an explicit empty --uid flag.
func launchArgs(goos string) []string {
	if goos == "linux" {
		return []string{"--uid="}
	}
	return nil
}

// sessionMode maps AO's approval vocabulary onto Antigravity's mode ids.
// The separate antigravity-acp server has default, auto_edit, and yolo, but no
// native Auto mode. AO Auto therefore uses auto_edit, leaving commands behind
// ACP permission requests; explicit bypass selects yolo. Default asks for all
// gated tools.
func sessionMode(permission ports.PermissionMode) string {
	switch ports.NormalizePermissionMode(permission) {
	case ports.PermissionModeDefault:
		return modeDefault
	case ports.PermissionModeAcceptEdits:
		return modeAutoEdit
	case ports.PermissionModeAuto:
		return modeAutoEdit
	case ports.PermissionModeBypassPermissions:
		return modeYolo
	default:
		return ""
	}
}

func sessionOptions(settings ports.ChatTurnSettings) []acpdriver.SessionOption {
	if model := strings.TrimSpace(settings.Model); model != "" {
		return []acpdriver.SessionOption{{ID: "model", Value: model}}
	}
	return nil
}

// sessionPreamble delivers AO's standing instructions as a text block ahead of
// the first prompt. Antigravity accepts no system-prompt launch flag, and it
// treats an embedded resource as something to look up (it web-searched the
// resource URI in testing) rather than as instructions, so plain text is the
// channel it follows. The instructions do not depend on any mode the user can
// switch, so they survive approval changes.
func sessionPreamble(cfg acpdriver.LaunchConfig) []acpsdk.ContentBlock {
	standing := strings.TrimSpace(cfg.SystemPrompt)
	if standing == "" {
		return nil
	}
	return []acpsdk.ContentBlock{acpsdk.TextBlock(
		"Standing instructions from Agent Orchestrator. Follow them for this whole session:\n" +
			"<ao_standing_instructions>\n" + standing + "\n</ao_standing_instructions>",
	)}
}
