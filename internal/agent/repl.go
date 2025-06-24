package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/chzyer/readline"
	"github.com/mark3labs/mcp-go/mcp"
)

// REPL represents the Read-Eval-Print Loop for MCP interaction
type REPL struct {
	client           *Client
	logger           *Logger
	rl               *readline.Instance
	notificationChan chan mcp.JSONRPCNotification
	stopChan         chan struct{}
	wg               sync.WaitGroup
}

// NewREPL creates a new REPL instance
func NewREPL(client *Client, logger *Logger) *REPL {
	return &REPL{
		client:           client,
		logger:           logger,
		notificationChan: make(chan mcp.JSONRPCNotification, 10),
		stopChan:         make(chan struct{}),
	}
}

// Run starts the REPL
func (r *REPL) Run(ctx context.Context) error {
	r.logger.Info("Connecting to MCP aggregator at %s using %s transport...", r.client.endpoint, r.client.transport)

	// Create and connect MCP client
	mcpClient, notificationChan, err := r.client.createAndConnectClient(ctx)
	if err != nil {
		return err
	}
	defer mcpClient.Close()

	r.client.client = mcpClient

	// Set up REPL-specific notification channel routing for SSE
	if r.client.transport == TransportSSE && notificationChan != nil {
		go func() {
			for notification := range notificationChan {
				select {
				case r.notificationChan <- notification:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Initialize session and load initial data
	if err := r.client.initializeAndLoadData(ctx); err != nil {
		return err
	}

	// Set up readline with tab completion
	completer := r.createCompleter()
	historyFile := filepath.Join(os.TempDir(), ".envctl_agent_history")

	config := &readline.Config{
		Prompt:          "MCP> ",
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",

		HistorySearchFold:   true,
		FuncFilterInputRune: filterInput,
	}

	rl, err := readline.NewEx(config)
	if err != nil {
		return fmt.Errorf("failed to create readline instance: %w", err)
	}
	defer rl.Close()
	r.rl = rl

	// Start notification listener in background (only for SSE transport)
	if r.client.transport == TransportSSE {
		r.wg.Add(1)
		go r.notificationListener(ctx)
		r.logger.Info("MCP REPL started with notification support. Type 'help' for available commands. Use TAB for completion.")
	} else {
		r.logger.Info("MCP REPL started. Type 'help' for available commands. Use TAB for completion.")
		r.logger.Info("Note: Real-time notifications are not supported with %s transport.", r.client.transport)
	}
	fmt.Println()

	// Main REPL loop
	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			if r.client.transport == TransportSSE {
				close(r.stopChan)
				r.wg.Wait()
			}
			r.logger.Info("REPL shutting down...")
			return nil
		default:
		}

		// Read input
		line, err := r.rl.Readline()
		if err == readline.ErrInterrupt {
			if len(line) == 0 {
				continue
			}
		} else if err == io.EOF {
			if r.client.transport == TransportSSE {
				close(r.stopChan)
				r.wg.Wait()
			}
			r.logger.Info("Goodbye!")
			return nil
		} else if err != nil {
			return fmt.Errorf("readline error: %w", err)
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		// Parse and execute command
		if err := r.executeCommand(ctx, input); err != nil {
			if err.Error() == "exit" {
				if r.client.transport == TransportSSE {
					close(r.stopChan)
					r.wg.Wait()
				}
				r.logger.Info("Goodbye!")
				return nil
			}
			r.logger.Error("Error: %v", err)
		}

		fmt.Println()
	}
}



// notificationListener handles notifications in the background
func (r *REPL) notificationListener(ctx context.Context) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case notification := <-r.notificationChan:
			// Temporarily pause readline
			if r.rl != nil {
				r.rl.Stdout().Write([]byte("\r\033[K"))
			}

			// Handle the notification (this will log it)
			if err := r.client.handleNotification(ctx, notification); err != nil {
				r.logger.Error("Failed to handle notification: %v", err)
			}

			// Update completer if items changed
			switch notification.Method {
			case "notifications/tools/list_changed",
				"notifications/resources/list_changed",
				"notifications/prompts/list_changed":
				if r.rl != nil {
					r.rl.Config.AutoComplete = r.createCompleter()
				}
			}

			// Refresh readline prompt
			if r.rl != nil {
				r.rl.Refresh()
			}
		}
	}
}








