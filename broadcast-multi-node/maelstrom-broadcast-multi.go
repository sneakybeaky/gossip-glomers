package main

import (
	"encoding/json"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log/slog"
	"os"
	"sync"
)

type app struct {
	mx         sync.RWMutex
	messages   map[float64]bool
	n          *maelstrom.Node
	neighbours []string
	logger     *slog.Logger
}

func newApp(n *maelstrom.Node) *app {

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	a := &app{
		n:        n,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, opts)),
		messages: make(map[float64]bool),
	}

	n.Handle("broadcast", a.handleBroadcast)
	n.Handle("read", a.handleRead)
	n.Handle("topology", a.handleTopology)
	n.Handle("init", a.handleInit)

	return a
}

func (a *app) handleInit(msg maelstrom.Message) error {
	var b maelstrom.InitMessageBody
	if err := json.Unmarshal(msg.Body, &b); err != nil {
		return err
	}

	a.logger = a.logger.With("id", b.NodeID)
	return nil
}

func (a *app) handleBroadcast(msg maelstrom.Message) error {

	type broadcast struct {
		Type    string  `json:"type"`
		Message float64 `json:"message"`
	}

	var body broadcast
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	a.mx.Lock()
	defer a.mx.Unlock()
	a.messages[body.Message] = true

	// Send to adjacent nodes to us
	for _, neighbour := range a.neighbours {
		a.logger.Debug("Sending broadcast to neighbour", slog.String("neighbour", neighbour))
		err := a.n.RPC(neighbour, body, func(msg maelstrom.Message) error {
			//a.logger.Debug("Got callback", slog.Any("msg", msg))
			return nil
		})
		if err != nil {
			a.logger.Error("Failed to send to neighbour",
				slog.String("neighbour", neighbour),
				slog.Any("error", err))
			return err
		}
	}

	response := map[string]any{
		"type": "broadcast_ok",
	}

	return a.n.Reply(msg, response)
}

func (a *app) handleRead(msg maelstrom.Message) error {
	a.mx.RLock()
	defer a.mx.RUnlock()

	a.logger.Debug("Returning messages I've seen", slog.Int("count", len(a.messages)))

	messages := make([]float64, len(a.messages))

	i := 0
	for k, _ := range a.messages {
		messages[i] = k
		i++
	}

	response := map[string]any{
		"type":     "read_ok",
		"messages": messages,
	}

	return a.n.Reply(msg, response)

}

func (a *app) handleTopology(msg maelstrom.Message) error {

	type topology struct {
		Topology map[string][]string `json:"topology"`
	}

	var t topology
	if err := json.Unmarshal(msg.Body, &t); err != nil {
		return err
	}

	a.logger.Info("Topology", slog.Any("topology", t.Topology))

	a.mx.Lock()
	defer a.mx.Unlock()
	a.neighbours = t.Topology[a.n.ID()]

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
