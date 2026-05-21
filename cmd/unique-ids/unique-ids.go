package main

import (
	"encoding/json"
	"fmt"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log/slog"
	"sync/atomic"
)

func main() {
	n := maelstrom.NewNode()

	i := atomic.Int64{}

	n.Handle("generate", func(msg maelstrom.Message) error {
		// Unmarshal the message body as a loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "generate_ok"
		body["id"] = fmt.Sprintf("%s:%d", n.ID(), i.Add(1))

		// Echo the original message back with the updated message type and content.
		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		slog.Error("Unable to run the handler", slog.Any("error", err))
	}

}
