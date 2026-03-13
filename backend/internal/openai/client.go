package openai

import (
	"context"
	"fmt"

	gogpt "github.com/sashabaranov/go-openai"
)

type Client struct {
	inner *gogpt.Client
}

func New(apiKey string) *Client {
	return &Client{inner: gogpt.NewClient(apiKey)}
}

// Chat sends a system prompt + user message and returns the assistant reply.
func (c *Client) Chat(ctx context.Context, systemPrompt string, userMessage string) (string, error) {
	resp, err := c.inner.CreateChatCompletion(ctx, gogpt.ChatCompletionRequest{
		Model: gogpt.GPT4oMini,
		Messages: []gogpt.ChatCompletionMessage{
			{Role: gogpt.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: gogpt.ChatMessageRoleUser, Content: userMessage},
		},
	})
	if err != nil {
		return "", fmt.Errorf("openai chat: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai: no choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}
