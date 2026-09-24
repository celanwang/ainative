package modelapi

import (
	"context"
	"encoding/json"
)

const chatCompletionsPath = "/chat/completions"

// Message 是一轮对话。
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 是一次聊天补全。
type ChatRequest struct {
	Model          string          `json:"model,omitempty"`
	Messages       []Message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    *float64        `json:"temperature,omitempty"`
	EnableThinking *bool           `json:"enable_thinking,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	StreamOptions  *StreamOptions  `json:"stream_options,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// StreamOptions 让流失输出模块带上用量。
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ResponseFormat 约束输出格式。
type ResponseFormat struct {
	Type string `json:"type"`
}

// ChatCompletion 是同步响应。
type ChatCompletion struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   *ChatUsage   `json:"usage"`
}

// ChatChoice 是一个候选。是否答完看 FinishReason。
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatMessage 是模型回复。ReasoningContent 仅在开启思考时有值。
type ChatMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

// ChatUsage 用 prompt/completion 计 token。推理 token 算在 completion 里。
type ChatUsage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

// ChatChunk 是一块增量。结束前 FinishReason 为空。
type ChatChunk struct {
	Object  string `json:"object"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Delta        struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *ChatUsage `json:"usage"`
}

// 无状态。历史由调用方重发。
func (c *Client) CreateChatCompletion(ctx context.Context, req ChatRequest) (ChatCompletion, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	req.Stream = false
	resp, err := c.post(ctx, chatCompletionsPath, req)
	if err != nil {
		return ChatCompletion{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatCompletion{}, readAPIError(resp)
	}
	defer resp.Body.Close()
	var out ChatCompletion
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatCompletion{}, err
	}
	return out, nil
}

// ChatStream 是流式收口后的结果。Events 保留每条 SSE 的 data，含末尾的 [DONE]。
type ChatStream struct {
	Text         string
	FinishReason string
	Usage        *ChatUsage
	Events       []string
}

// 拼接 delta。用量在末块。
func (c *Client) StreamChatCompletion(ctx context.Context, req ChatRequest) (ChatStream, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	req.Stream = true
	resp, err := c.post(ctx, chatCompletionsPath, req)
	if err != nil {
		return ChatStream{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ChatStream{}, readAPIError(resp)
	}
	defer resp.Body.Close()

	var acc ChatStream
	err = readSSE(resp.Body, func(ev SSEEvent) error {
		acc.Events = append(acc.Events, ev.Data)
		if ev.Data == "[DONE]" {
			return errStop
		}
		var chunk ChatChunk
		if err := json.Unmarshal([]byte(ev.Data), &chunk); err != nil {
			return err
		}
		for _, choice := range chunk.Choices {
			acc.Text += choice.Delta.Content
			if choice.FinishReason != "" {
				acc.FinishReason = choice.FinishReason
			}
		}
		if chunk.Usage != nil {
			acc.Usage = chunk.Usage
		}
		return nil
	})
	if err != nil {
		return ChatStream{}, err
	}
	return acc, nil
}
