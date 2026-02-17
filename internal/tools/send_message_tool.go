package tools

import (
	"context"
	"fmt"
	"io"
	"os"
)

// ChannelMessageSender sends a plain text message to the active user channel.
type ChannelMessageSender interface {
	Send(ctx context.Context, message string) error
}

// SendMessageTool delivers assistant text to the current channel.
type SendMessageTool struct {
	Sender ChannelMessageSender
	Writer io.Writer
}

// Name returns the tool name.
func (t SendMessageTool) Name() string {
	return "send_message"
}

// Description returns the tool description for the model.
func (t SendMessageTool) Description() string {
	return "Send a message to the active user channel"
}

// Schema returns the JSON schema for send_message args.
func (t SendMessageTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Message text to send",
			},
			"channel": map[string]any{
				"type":        "string",
				"description": "Optional channel identifier (unused in MVP)",
			},
		},
		"required": []string{"message"},
	}
}

// Permission declares default permission behavior for this tool.
func (t SendMessageTool) Permission() Permission {
	return AutoApprove
}

// Execute sends a message through Sender or falls back to writing to Writer/stdout.
func (t SendMessageTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	message, err := stringArg(args, "message")
	if err != nil {
		return nil, err
	}

	if t.Sender != nil {
		if err := t.Sender.Send(ctx, message); err != nil {
			return nil, err
		}
		return &ToolResult{Output: "sent"}, nil
	}

	writer := t.Writer
	if writer == nil {
		writer = os.Stdout
	}
	if _, err := fmt.Fprintln(writer, message); err != nil {
		return nil, fmt.Errorf("write message output: %w", err)
	}
	return &ToolResult{Output: "sent"}, nil
}
