package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/failsafe-go/failsafe-go"
	"github.com/failsafe-go/failsafe-go/retrypolicy"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	"golang.org/x/sync/errgroup"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"
)

type app struct {
	mx         sync.RWMutex
	messages   map[float64]bool
	n          *maelstrom.Node
	neighbours []string
	logger     *slog.Logger
	ctx        context.Context
}

func newApp(ctx context.Context, n *maelstrom.Node) *app {

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	a := &app{
		n:        n,
		logger:   slog.New(slog.NewTextHandler(os.Stderr, opts)),
		messages: make(map[float64]bool),
		ctx:      ctx,
	}

	n.Handle("broadcast", a.handleBroadcast)
	n.Handle("read", a.handleRead)
	n.Handle("topology", a.handleTopology)
	n.Handle("init", a.handleInit)

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
	rpc         func(context.Context, string, any) (maelstrom.Message, error)
	retryPolicy retrypolicy.RetryPolicy[maelstrom.Message]
}

func (b broadcaster) broadcast(ctx context.Context) error {

	type responseBody struct {
		Type string `json:"type"`
	}

	b.logger.Debug("Sending broadcast to neighbour")

	resp, err := failsafe.Get[maelstrom.Message](func() (maelstrom.Message, error) {
		ctxt, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		defer cancel()
		return b.rpc(ctxt, b.dest, b.body)
	}, b.retryPolicy)

	if err != nil {
		b.logger.Error("Failed broadcasting", slog.Any("error", err))
		return err
	}

	b.logger.Debug("Got broadcast response", slog.Any("response", resp))

	var body responseBody
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return err
	}

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

	ok, _ := a.messages[body.Message]

	if ok {
		// already seen
		return nil
	}

	a.messages[body.Message] = true

	// Send to adjacent nodes to us
	for _, neighbour := range a.neighbours {

		go func(neighbour string) {
			b := broadcaster{
				dest:   neighbour,
				body:   body,
				logger: a.logger.With("destination", neighbour),
				rpc:    a.n.SyncRPC,
				retryPolicy: retrypolicy.Builder[maelstrom.Message]().
					HandleErrors(context.DeadlineExceeded).
					WithDelay(5 * time.Millisecond).WithMaxRetries(-1).Build(),
			}
			_ = b.broadcast(a.ctx)
		}(neighbour)

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
	for k := range a.messages {
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {

		n := maelstrom.NewNode()
		return newApp(gctx, n).Run()

	})

	err := g.Wait()
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Println("context was canceled")
		} else {
			log.Printf("received error: %v\n", err)
		}
	} else {
		log.Println("finished clean")
	}

}
