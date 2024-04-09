package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"log/slog"
	"os"
	"sync"
	"time"
)

type messages struct {
	mx       sync.RWMutex
	messages map[float64]bool
}

func newMessages() *messages {
	return &messages{
		messages: make(map[float64]bool),
	}
}

func (m *messages) Store(values ...float64) {
	m.mx.Lock()
	defer m.mx.Unlock()

	for _, v := range values {
		m.messages[v] = true
	}

}

func (m *messages) Messages() []float64 {
	m.mx.RLock()
	defer m.mx.RUnlock()

	var messages = make([]float64, len(m.messages))
	i := 0
	for k := range m.messages {
		messages[i] = k
		i++
	}

	return messages
}

func (m *messages) Seen(message float64) bool {
	m.mx.RLock()
	defer m.mx.RUnlock()
	seen, _ := m.messages[message]
	return seen
}

type app struct {
	messages   *messages
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
		messages: newMessages(),
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

type broadcaster struct {
	dest        string
	body        any
	logger      *slog.Logger
	send        func(dest string, body any) error
	retryPolicy retrypolicy.RetryPolicy[any]
}

func (b broadcaster) broadcast() error {

	b.logger.Debug("Sending sync to neighbour")

	err := failsafe.Run(func() error {
		return b.send(b.dest, b.body)
	}, b.retryPolicy)

	if err != nil {
		b.logger.Error("Failed broadcasting", slog.Any("error", err))
		return err
	}

	return nil
}

func (a *app) handleBroadcast(msg maelstrom.Message) error {

	type broadcast struct {
		Type    string  `json:"type"`
		Message float64 `json:"message"`
		History []string
	}

	var body broadcast
	if err := json.Unmarshal(msg.Body, &body); err != nil {
		return err
	}

	if a.messages.Seen(body.Message) {
		// already seen
		return nil
	}

	a.messages.Store(body.Message)

	sb := syncBody{
		Type:   "sync",
		Values: a.messages.Messages(),
	}

	// Send to adjacent nodes to us
	for _, neighbour := range a.neighbours {

		if neighbour == msg.Src {
			continue // don't send back to origin
		}

		go func(neighbour string, body any) {
			b := broadcaster{
				dest:   neighbour,
				body:   body,
				logger: a.logger.With("destination", neighbour),
				send:   a.n.Send,
				retryPolicy: retrypolicy.Builder[any]().
					HandleErrors(context.DeadlineExceeded).
					WithDelay(5 * time.Millisecond).WithMaxRetries(-1).Build(),
			}
			err := b.broadcast()

			if err != nil {
				a.logger.Error("Unable to broadcast",
					slog.String("destination", neighbour),
					slog.Any("error", err))
			}

		}(neighbour, sb)

	}

	response := map[string]any{
		"type": "broadcast_ok",
	}

	return a.n.Reply(msg, response)
}

func (a *app) handleRead(msg maelstrom.Message) error {

	a.logger.Debug("Returning messages I've seen")

	response := map[string]any{
		"type":     "read_ok",
		"messages": a.messages.Messages(),
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

	a.neighbours = t.Topology[a.n.ID()]

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

	a.messages.Store(s.Values...)

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
