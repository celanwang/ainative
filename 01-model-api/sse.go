package modelapi

import (
	"bufio"
	"errors"
	"io"
	"strings"
)

// SSEEvent 是一条以空行结束的服务端事件。
type SSEEvent struct {
	ID    string
	Event string
	Data  string
}

var errStop = errors.New("stop")

// readSSE 按空行切分事件。注释行被忽略，多行 data 按换行拼回。
func readSSE(r io.Reader, handle func(SSEEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 2<<20)

	var cur SSEEvent
	var data []string
	pending := false
	flush := func() error {
		if !pending {
			return nil
		}
		cur.Data = strings.Join(data, "\n")
		err := handle(cur)
		cur = SSEEvent{}
		data = data[:0]
		pending = false
		return err
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				if errors.Is(err, errStop) {
					return nil
				}
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		pending = true
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "id":
			cur.ID = value
		case "event":
			cur.Event = value
		case "data":
			data = append(data, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil && !errors.Is(err, errStop) {
		return err
	}
	return nil
}
