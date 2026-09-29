package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Johnny008-creator/offcode/internal/backend"
	"github.com/Johnny008-creator/offcode/internal/config"
	"github.com/Johnny008-creator/offcode/internal/permission"
	"github.com/Johnny008-creator/offcode/internal/tools"
)

const systemPrompt = `You are offcode, a local coding assistant running against an on-device model.
You have tools to read, write, and edit files, run shell commands, and search the codebase.
Use tools when you need real information instead of guessing. Keep responses concise.`

const maxToolIterationsPerTurn = 25

func Run(cfg config.Config) error {
	client := backend.NewClient(cfg.BaseURL)
	messages := []backend.Message{{Role: "system", Content: systemPrompt}}

	in := bufio.NewReader(os.Stdin)
	fmt.Printf("offcode — backend=%s model=%s mode=%s\n", cfg.Backend, cfg.Model, cfg.Mode)
	fmt.Println("Type your request, or /exit to quit.")

	for {
		fmt.Print("\n> ")
		line, err := in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if line == "/exit" {
			return nil
		}

		messages = append(messages, backend.Message{Role: "user", Content: line})

		if err := turn(context.Background(), client, cfg, &messages, in); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
}

func turn(ctx context.Context, client *backend.Client, cfg config.Config, messages *[]backend.Message, in *bufio.Reader) error {
	for i := 0; i < maxToolIterationsPerTurn; i++ {
		msg, err := client.ChatCompletion(ctx, backend.ChatRequest{
			Model:    cfg.Model,
			Messages: *messages,
			Tools:    tools.Definitions(),
		})
		if err != nil {
			return fmt.Errorf("backend request: %w", err)
		}
		*messages = append(*messages, *msg)

		if len(msg.ToolCalls) == 0 {
			fmt.Println(msg.Content)
			return nil
		}

		for _, tc := range msg.ToolCalls {
			result := runTool(cfg.Mode, tc, in)
			*messages = append(*messages, backend.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}
	return fmt.Errorf("stopped after %d tool iterations without a final answer", maxToolIterationsPerTurn)
}

func runTool(mode permission.Mode, tc backend.ToolCall, in *bufio.Reader) string {
	name := tc.Function.Name
	args := tc.Function.Arguments

	decision := permission.Check(mode, name, args)
	if decision.Denied {
		return "DENIED: " + decision.DenyReason
	}

	if decision.NeedsConfirm {
		fmt.Printf("\n[%s] %s\n", name, args)
		fmt.Print("Allow? [y/N] ")
		reply, _ := in.ReadString('\n')
		reply = strings.TrimSpace(strings.ToLower(reply))
		if reply != "y" && reply != "yes" {
			return "DENIED: user declined confirmation"
		}
	}

	out, err := tools.Dispatch(name, args)
	if err != nil {
		if out != "" {
			return fmt.Sprintf("error: %v\noutput:\n%s", err, out)
		}
		return fmt.Sprintf("error: %v", err)
	}
	return out
}
