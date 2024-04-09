package main

import (
	"encoding/json"
	"fmt"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log/slog"
	"os"
)

type app struct {
	values   *values
	n        *maelstrom.Node
	logger   *slog.Logger
	topology *topology
}

func newApp(n *maelstrom.Node) *app {

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	a := &app{
		n:      n,
		logger: slog.New(slog.NewTextHandler(os.Stderr, opts)),
		values: newValues(),
	}

	n.Handle("broadcast", a.handleBroadcast)
	n.Handle("read", a.handleRead)
	n.Handle("topology", a.handleTopology)
	n.Handle("init", a.handleInit)
	n.Handle("sync", a.handleSync)

	return a
}

func (a *app) Run() error {
	return a.n.Run()
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
		History []string
	}

	var body broadcast
	err := json.Unmarshal(msg.Body, &body)

	if err != nil {
		return err
	}

	response := map[string]any{
		"type": "broadcast_ok",
	}

	// Send back response now
	err = a.n.Reply(msg, response)
	if err != nil {
		return err
	}

	a.values.Store(body.Message)

	sb := syncBody{
		Type:   "sync",
		Values: a.values.Values(),
	}

	// Gossip with every other node to replicate our values
	for _, n := range a.topology.AllBut(a.n.ID()) {

		go func(destination string, body syncBody) {
			b := syncer{
				dest:   destination,
				body:   body,
				logger: a.logger.With("destination", destination),
				send:   a.n.Send,
			}
			err := b.sync()

			if err != nil {
				a.logger.Error("Unable to sync",
					slog.String("destination", destination),
					slog.Any("error", err))
			}

		}(n, sb)

	}

	return nil
}

func (a *app) handleRead(msg maelstrom.Message) error {

	a.logger.Debug("Returning values I've seen")

	response := map[string]any{
		"type":     "read_ok",
		"messages": a.values.Values(),
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

	a.topology = newTopology(t.Topology)

	response := map[string]any{
		"type": "topology_ok",
	}

	return a.n.Reply(msg, response)

}

type syncBody struct {
	Type   string    `json:"type"`
	Values []float64 `json:"values"`
}

func (a *app) handleSync(msg maelstrom.Message) error {
	var s syncBody
	if err := json.Unmarshal(msg.Body, &s); err != nil {
		return err
	}

	a.values.Store(s.Values...)

	return nil
}

func main() {

	n := maelstrom.NewNode()
	err := newApp(n).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run app : %+v", err)
		os.Exit(1)
	}

}
