package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log/slog"
	"sync"
)

type app struct {
	mx       sync.RWMutex
	messages []float64
	n        *maelstrom.Node
}

func newApp(n *maelstrom.Node) *app {
	a := &app{
		n: n,
	}

	n.Handle("broadcast", a.broadcast)
	n.Handle("read", a.read)
	n.Handle("topology", a.topology)

	return a
}

func (a *app) broadcast(msg maelstrom.Message) error {
	// Unmarshal the message body as a loosely-typed map.
	var body map[string]any
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	message := body["message"].(float64)

	a.mx.Lock()
	defer a.mx.Unlock()
	a.messages = append(a.messages, message)

	response := map[string]any{
		"type": "broadcast_ok",
	}

	return a.n.Reply(msg, response)
}

func (a *app) read(msg maelstrom.Message) error {
	a.mx.Lock()
	defer a.mx.Unlock()

	response := map[string]any{
		"type":     "read_ok",
		"messages": a.messages,
	}

	return a.n.Reply(msg, response)

}

func (a *app) topology(msg maelstrom.Message) error {
	a.mx.Lock()
	defer a.mx.Unlock()

	response := map[string]any{
		"type": "topology_ok",
	}

	return a.n.Reply(msg, response)

}

func main() {

	n := maelstrom.NewNode()
	newApp(n)

	if err := n.Run(); err != nil {
		slog.Error("Unable to run the handler", slog.Any("error", err))
	}

}
