package main

import (
	cryptorand "crypto/rand"
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"github.com/oklog/ulid/v2"
	"log/slog"
	"time"
)

func main() {
	n := maelstrom.NewNode()

	entropy := cryptorand.Reader

	n.Handle("generate", func(msg maelstrom.Message) error {
		// Unmarshal the message body as a loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		// Update the message type to return back.
		body["type"] = "generate_ok"

		ms := ulid.Timestamp(time.Now())
		id, err := ulid.New(ms, entropy)
		if err != nil {
			return err
		}

		body["id"] = id

		// Echo the original message back with the updated message type and content.
		return n.Reply(msg, body)
	})

	if err := n.Run(); err != nil {
		slog.Error("Unable to run the handler", slog.Any("error", err))
	}

}
