package modelapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	return NewClient(cfg)
}

func thinkingOff() *bool {
	v := false
	return &v
}

func TestChatCompletionShape(t *testing.T) {
	client := testClient(t)
	got, err := client.CreateChatCompletion(context.Background(), ChatRequest{
		Messages:       []Message{{Role: "user", Content: "Reply with the single word OK."}},
		MaxTokens:      32,
		EnableThinking: thinkingOff(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Object != "chat.completion" {
		t.Fatalf("object = %q", got.Object)
	}
	if got.Model != client.Model {
		t.Fatalf("model = %q, want %q", got.Model, client.Model)
	}
	if len(got.Choices) != 1 {
		t.Fatalf("choices = %d", len(got.Choices))
	}
	choice := got.Choices[0]
	if choice.Message.Role != "assistant" || strings.TrimSpace(choice.Message.Content) == "" {
		t.Fatalf("message = %+v", choice.Message)
	}
	if choice.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", choice.FinishReason)
	}
	if got.Usage == nil || got.Usage.PromptTokens == 0 || got.Usage.CompletionTokens == 0 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestChatCompletionLengthStop(t *testing.T) {
	client := testClient(t)
	got, err := client.CreateChatCompletion(context.Background(), ChatRequest{
		Messages:       []Message{{Role: "user", Content: "Count from 1 to 30 separated by spaces."}},
		MaxTokens:      1,
		EnableThinking: thinkingOff(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Choices[0].FinishReason != "length" {
		t.Fatalf("finish_reason = %q, content = %q", got.Choices[0].FinishReason, got.Choices[0].Message.Content)
	}
	if got.Usage == nil || got.Usage.CompletionTokens == 0 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestChatCompletionJSONObject(t *testing.T) {
	client := testClient(t)
	got, err := client.CreateChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{
			Role:    "user",
			Content: "Return a JSON object with key ok and boolean value true. Output JSON only.",
		}},
		MaxTokens:      64,
		EnableThinking: thinkingOff(),
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", got.Choices[0].FinishReason)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(got.Choices[0].Message.Content), &payload); err != nil {
		t.Fatalf("content is not JSON: %q (%v)", got.Choices[0].Message.Content, err)
	}
}

func TestChatStreamAssembles(t *testing.T) {
	client := testClient(t)
	got, err := client.StreamChatCompletion(context.Background(), ChatRequest{
		Messages:       []Message{{Role: "user", Content: "Reply with the single word OK."}},
		MaxTokens:      16,
		EnableThinking: thinkingOff(),
		StreamOptions:  &StreamOptions{IncludeUsage: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.Text) == "" {
		t.Fatal("empty stream text")
	}
	if got.FinishReason != "stop" {
		t.Fatalf("finish_reason = %q", got.FinishReason)
	}
	if got.Usage == nil || got.Usage.TotalTokens == 0 {
		t.Fatalf("usage = %+v", got.Usage)
	}
	if len(got.Events) == 0 || got.Events[len(got.Events)-1] != "[DONE]" {
		t.Fatalf("events = %v", got.Events)
	}
}
