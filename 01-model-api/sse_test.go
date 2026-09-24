package modelapi

import (
	"strings"
	"testing"
)

func TestReadSSE(t *testing.T) {
	raw := strings.Join([]string{
		":HTTP_STATUS/200",
		"id:1",
		"event:response.output_text.delta",
		`data:{"delta":"O"}`,
		"",
		"event:response.output_text.delta",
		`data:{"delta":"K`,
		"data:.}",
		"",
		"data: [DONE]",
		"",
	}, "\n")

	var got []SSEEvent
	err := readSSE(strings.NewReader(raw), func(ev SSEEvent) error {
		got = append(got, ev)
		if ev.Data == "[DONE]" {
			return errStop
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("events = %+v", got)
	}
	if got[0].ID != "1" || got[0].Event != "response.output_text.delta" || got[0].Data != `{"delta":"O"}` {
		t.Fatalf("first = %+v", got[0])
	}
	if got[1].Data != "{\"delta\":\"K\n.}" {
		t.Fatalf("multiline data = %q", got[1].Data)
	}
	if got[2].Event != "" || got[2].Data != "[DONE]" {
		t.Fatalf("done = %+v", got[2])
	}
}
