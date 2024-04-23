package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
	stdlog "log"

	"os"
)

type app struct {
	n   *maelstrom.Node
	log logr.Logger
	kv  *maelstrom.KV
}

func newApp(n *maelstrom.Node, log logr.Logger) *app {

	a := &app{
		n:   n,
		log: log,
		kv:  maelstrom.NewSeqKV(n),
	}

	n.Handle("add", a.handleAdd)
	n.Handle("read", a.handleRead)
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

	a.log = a.log.WithName(b.NodeID)
	return nil
}

func (a *app) handleRead(msg maelstrom.Message) error {

	a.log.V(1).Info("Returning value")

	type ReadOkMessageBody struct {
		maelstrom.MessageBody
		Value int64 `json:"value"`
	}

	var counter int64
	err := a.kv.ReadInto(context.Background(), "counter", &counter)

	if err != nil {
		a.log.Error(err, "Unable to read counter")
		return a.n.Reply(msg, maelstrom.NewRPCError(maelstrom.TemporarilyUnavailable, "Unable to read counter from kv store"))
	}

	a.log.V(1).Info("Read counter", "counter", counter)

	b := ReadOkMessageBody{
		MessageBody: maelstrom.MessageBody{
			Type: "read_ok",
		},
		Value: counter,
	}

	return a.n.Reply(msg, b)

}

func (a *app) handleAdd(msg maelstrom.Message) error {

	type AddMessageBody struct {
		maelstrom.MessageBody
		Delta int64 `json:"delta"`
	}

	var b AddMessageBody

	if err := json.Unmarshal(msg.Body, &b); err != nil {
		return err
	}

	a.log.V(1).Info("Adding delta", "delta", b.Delta)

	var counter int64
	err := a.kv.ReadInto(context.Background(), "counter", &counter)

	if err != nil {

		a.log.Error(err, "Problem reading counter from kv")

		var rpcError *maelstrom.RPCError
		if errors.As(err, &rpcError) {

			if rpcError.Code != maelstrom.KeyDoesNotExist {
				a.log.Error(rpcError, "RPC error reading from kv", "code", rpcError.Code, "text", rpcError.Text)
				return a.n.Reply(msg, maelstrom.NewRPCError(maelstrom.Crash, "Unable to read counter from kv store"))
			}

		}
	}

	a.log.V(1).Info("Read counter ok", "counter", counter)

	err = a.kv.CompareAndSwap(context.Background(), "counter", counter, counter+b.Delta, true)
	if err != nil {
		a.log.Error(err, "Problem setting counter in kv")

		var rpcError *maelstrom.RPCError
		if errors.As(err, &rpcError) {

			a.log.Error(rpcError, "RPC error writing to kv store", "code", rpcError.Code, "text", rpcError.Text)

			return a.n.Reply(msg, maelstrom.NewRPCError(maelstrom.Crash, "Unable to write counter to kv store"))

		}

	}

	return a.n.Reply(msg, maelstrom.MessageBody{
		Type: "add_ok",
	})

}

func main() {

	stdr.SetVerbosity(3)
	log := stdr.NewWithOptions(stdlog.New(os.Stderr, "", stdlog.LstdFlags), stdr.Options{LogCaller: stdr.All})

	n := maelstrom.NewNode()
	err := newApp(n, log).Run()
	if err != nil {
		log.Error(err, "Failed to start app")
		os.Exit(1)
	}

}
