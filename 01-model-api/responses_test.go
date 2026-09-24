package modelapi

import (
	"context"
	"strings"
	"testing"
)

func none() *Reasoning { return &Reasoning{Effort: "none"} }

func TestResponseTextProjection(t *testing.T) {
	client := testClient(t)
	got, err := client.CreateResponse(context.Background(), ResponseRequest{
		Input:           Input{Text: "Reply with the single word OK."},
		MaxOutputTokens: 16,
		Reasoning:       none(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Object != "response" || got.Status != "completed" {
		t.Fatalf("object=%q status=%q", got.Object, got.Status)
	}
	if got.Model != client.Model {
		t.Fatalf("model = %q, want %q", got.Model, client.Model)
	}
	if strings.TrimSpace(got.OutputText()) == "" {
		t.Fatalf("output = %+v", got.Output)
	}
	if got.Usage == nil || got.Usage.InputTokens == 0 || got.Usage.OutputTokens == 0 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestResponseReasoningPrecedesMessage(t *testing.T) {
	client := testClient(t)
	got, err := client.CreateResponse(context.Background(), ResponseRequest{
		Input:           Input{Text: "Which is larger, 9.9 or 9.11? Reply with the larger number only."},
		MaxOutputTokens: 128,
		Reasoning:       &Reasoning{Effort: "low"},
	})
	if err != nil {
		t.Fatal(err)
	}
	reasoningAt, messageAt := -1, -1
	for i, item := range got.Output {
		switch item.Type {
		case "reasoning":
			if reasoningAt < 0 {
				reasoningAt = i
			}
		case "message":
			if messageAt < 0 {
				messageAt = i
			}
		}
	}
	if reasoningAt < 0 || messageAt < 0 || reasoningAt >= messageAt {
		t.Fatalf("output order = %v", itemTypes(got.Output))
	}
	if strings.TrimSpace(got.OutputText()) == "" {
		t.Fatal("message text missing behind the reasoning item")
	}
	if got.Usage == nil || got.Usage.OutputTokensDetails.ReasoningTokens == 0 {
		t.Fatalf("usage = %+v", got.Usage)
	}
}

func TestPreviousResponseCarriesInputTokens(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()
	first, err := client.CreateResponse(ctx, ResponseRequest{
		Input:           Input{Text: "Remember this code: pine. Reply with OK."},
		MaxOutputTokens: 16,
		Reasoning:       none(),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.CreateResponse(ctx, ResponseRequest{
		Input:              Input{Text: "What code did I ask you to remember?"},
		MaxOutputTokens:    32,
		Reasoning:          none(),
		PreviousResponseID: first.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "completed" {
		t.Fatalf("status = %q", second.Status)
	}
	if first.Usage == nil || second.Usage == nil || second.Usage.InputTokens <= first.Usage.InputTokens {
		t.Fatalf("input tokens first=%v second=%v", first.Usage, second.Usage)
	}
	if !strings.Contains(strings.ToLower(second.OutputText()), "pine") {
		t.Fatalf("second text = %q", second.OutputText())
	}
}

func TestResponseStreamAssembles(t *testing.T) {
	client := testClient(t)
	got, err := client.StreamResponse(context.Background(), ResponseRequest{
		Input:           Input{Text: "Reply with the single word OK."},
		MaxOutputTokens: 16,
		Reasoning:       none(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.Text) == "" {
		t.Fatal("empty deltas")
	}
	if got.Response == nil || got.Response.Status != "completed" {
		t.Fatalf("completed = %+v", got.Response)
	}
	if got.Response.Usage == nil || got.Response.Usage.TotalTokens == 0 {
		t.Fatalf("usage = %+v", got.Response.Usage)
	}
	if got.Text != got.Response.OutputText() {
		t.Fatalf("deltas = %q, completed = %q", got.Text, got.Response.OutputText())
	}
	if !containsInOrder(got.EventTypes, "response.output_text.delta", "response.completed") {
		t.Fatalf("events = %v", got.EventTypes)
	}
}

func containsInOrder(items []string, want ...string) bool {
	i := 0
	for _, item := range items {
		if item == want[i] {
			i++
			if i == len(want) {
				return true
			}
		}
	}
	return false
}

func itemTypes(items []OutputItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Type
	}
	return out
}
