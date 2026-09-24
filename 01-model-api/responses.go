package modelapi

import (
	"context"
	"encoding/json"
	"strings"
)

const responsesPath = "/responses"

// Input 是本轮材料：字符串或消息数组。
type Input struct {
	Text  string
	Items []InputMessage
}

// 有 Items 则发数组，否则发字符串。
func (in Input) MarshalJSON() ([]byte, error) {
	if in.Items != nil {
		return json.Marshal(in.Items)
	}
	return json.Marshal(in.Text)
}

// InputMessage 是 input 数组里的一条消息。
type InputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Reasoning 设置思考强度。none 表示直接作答。
type Reasoning struct {
	Effort string `json:"effort"`
}

// ResponseRequest 是一次 Responses 调用。默认会思考，需显式设 effort。
type ResponseRequest struct {
	Model              string     `json:"model,omitempty"`
	Input              Input      `json:"input"`
	Instructions       string     `json:"instructions,omitempty"`
	MaxOutputTokens    int        `json:"max_output_tokens,omitempty"`
	Reasoning          *Reasoning `json:"reasoning,omitempty"`
	PreviousResponseID string     `json:"previous_response_id,omitempty"`
	Stream             bool       `json:"stream,omitempty"`
}

// Response 是同步响应。正文在 Output 里，结束状态看 Status。
type Response struct {
	ID     string       `json:"id"`
	Object string       `json:"object"`
	Model  string       `json:"model"`
	Status string       `json:"status"`
	Output []OutputItem `json:"output"`
	Usage  *Usage       `json:"usage"`
}

// OutputItem 是一次模型动作，如 reasoning 或 message。
type OutputItem struct {
	ID      string        `json:"id"`
	Type    string        `json:"type"`
	Role    string        `json:"role,omitempty"`
	Status  string        `json:"status,omitempty"`
	Content []ContentPart `json:"content,omitempty"`
	Summary []ContentPart `json:"summary,omitempty"`
}

// ContentPart 是一块内容。可见回答的类型是 output_text。
type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Usage 用 input/output 计 token。推理 token 算在 output 里。
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

// 只取 message 的 output_text，跳过 reasoning。
func (r Response) OutputText() string {
	var b strings.Builder
	for _, item := range r.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				b.WriteString(part.Text)
			}
		}
	}
	return b.String()
}

// instructions 不随 previous_response_id 继承。
func (c *Client) CreateResponse(ctx context.Context, req ResponseRequest) (Response, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	req.Stream = false
	resp, err := c.post(ctx, responsesPath, req)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Response{}, readAPIError(resp)
	}
	defer resp.Body.Close()
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, err
	}
	return out, nil
}

// ResponseEvent 是流式事件。Delta 只在文本增量事件上有值。
type ResponseEvent struct {
	Type     string    `json:"type"`
	Delta    string    `json:"delta,omitempty"`
	Response *Response `json:"response,omitempty"`
}

// ResponseStream 是拼好的文本、事件名，以及 completed 事件里的完整响应。
type ResponseStream struct {
	Text       string
	EventTypes []string
	Response   *Response
}

// 以 completed 事件为准。
func (c *Client) StreamResponse(ctx context.Context, req ResponseRequest) (ResponseStream, error) {
	if req.Model == "" {
		req.Model = c.Model
	}
	req.Stream = true
	resp, err := c.post(ctx, responsesPath, req)
	if err != nil {
		return ResponseStream{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ResponseStream{}, readAPIError(resp)
	}
	defer resp.Body.Close()

	var acc ResponseStream
	err = readSSE(resp.Body, func(ev SSEEvent) error {
		var event ResponseEvent
		if err := json.Unmarshal([]byte(ev.Data), &event); err != nil {
			return err
		}
		name := ev.Event
		if name == "" {
			name = event.Type
		}
		acc.EventTypes = append(acc.EventTypes, name)
		switch name {
		case "response.output_text.delta":
			acc.Text += event.Delta
		case "response.completed":
			acc.Response = event.Response
			return errStop
		case "error":
			return &APIError{Status: resp.StatusCode, Body: ev.Data}
		}
		return nil
	})
	if err != nil {
		return ResponseStream{}, err
	}
	return acc, nil
}
