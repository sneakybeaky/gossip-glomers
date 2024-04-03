package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log/slog"
	"sync"
)

func main() {

	mutex := sync.RWMutex{}
	var messages []float64

	n := maelstrom.NewNode()

	n.Handle("broadcast", func(msg maelstrom.Message) error {

		// Unmarshal the message body as a loosely-typed map.
		var body map[string]any
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		message := body["message"].(float64)

		mutex.Lock()
		defer mutex.Unlock()
		messages = append(messages, message)

		response := map[string]any{
			"type": "broadcast_ok",
		}

		return n.Reply(msg, response)

	})

	n.Handle("read", func(msg maelstrom.Message) error {

		mutex.Lock()
		defer mutex.Unlock()

		response := map[string]any{
			"type":     "read_ok",
			"messages": messages,
		}

		return n.Reply(msg, response)
	})

	n.Handle("topology", func(msg maelstrom.Message) error {

		response := map[string]any{
			"type": "topology_ok",
		}

		return n.Reply(msg, response)
	})

	if err := n.Run(); err != nil {
		slog.Error("Unable to run the handler", slog.Any("error", err))
	}

}
