package tools

import (
	"context"
	"fmt"
	"io"
	"os"
)

type ChannelMessageSender interface {
	Send(ctx context.Context, message string) error
}

type SendMessageTool struct {
	Sender ChannelMessageSender
	Writer io.Writer
}

func (t SendMessageTool) Name() string {
	return "send_message"
}

func (t SendMessageTool) Description() string {
	return "Send a message to the active user channel"
}

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

func (t SendMessageTool) Permission() Permission {
	return AutoApprove
}

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
