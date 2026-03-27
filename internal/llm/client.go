package llm

import (
	"context"
	"fmt"
	"math"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// Result holds the LLM response content and usage statistics.
type Result struct {
	Content   string
	TokensIn  int
	TokensOut int
}

// Client wraps an OpenAI-compatible API client.
type Client struct {
	client       *openai.Client
	model        string
	maxRetries   int
	callTimeout  time.Duration
}

// NewClient creates a new LLM client.
func NewClient(apiKey, baseURL, model string) *Client {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = baseURL
	return &Client{
		client:      openai.NewClientWithConfig(config),
		model:       model,
		maxRetries:  3,
		callTimeout: 60 * time.Second,
	}
}

// Call sends a chat completion request with retry logic.
// It retries up to 3 times with exponential backoff: 1s, 2s, 4s.
func (c *Client) Call(ctx context.Context, systemPrompt, userPrompt string) (*Result, error) {
	var lastErr error
	for attempt := range c.maxRetries {
		result, err := c.doCall(ctx, systemPrompt, userPrompt)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if attempt < c.maxRetries-1 {
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", c.maxRetries, lastErr)
}

func (c *Client) doCall(ctx context.Context, systemPrompt, userPrompt string) (*Result, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()

	resp, err := c.client.CreateChatCompletion(callCtx, openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("API call: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("API returned no choices")
	}
	return &Result{
		Content:   resp.Choices[0].Message.Content,
		TokensIn:  resp.Usage.PromptTokens,
		TokensOut: resp.Usage.CompletionTokens,
	}, nil
}
