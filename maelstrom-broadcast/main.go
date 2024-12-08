package main

import (
	"encoding/json"
	"log"
	"sync"
	"sync/atomic"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()
	s := NewServer(n)

	n.Handle("broadcast", s.HandleBroadcast())
	n.Handle("read", s.HandleRead())
	n.Handle("topology", s.HandleTopology())

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	node       *maelstrom.Node
	broadcasts Broadcasts
	topology   atomic.Pointer[map[string][]string]
}

func NewServer(node *maelstrom.Node) *Server {
	server := &Server{
		node: node,
		broadcasts: Broadcasts{
			broadcasts: make(map[int]struct{}),
			cached:     make([]int, 0),
		},
	}

	go func() {
		// Background thread that propagates broadcasts through gossip protocol

	}()

	return server
}

type Broadcasts struct {
	mu         sync.RWMutex
	broadcasts map[int]struct{}
	cached     []int
}

func (b *Broadcasts) Add(message int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.broadcasts[message]; !ok {
		b.broadcasts[message] = struct{}{}
		b.cached = append(b.cached, message)
	}
}

func (b *Broadcasts) Read() []int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	broadcasts := make([]int, len(b.cached))
	copy(broadcasts, b.cached)

	return broadcasts
}

type Message struct {
	Type     string              `json:"type"`
	Message  int                 `json:"message,omitempty"`
	Topology map[string][]string `json:"topology,omitempty"`
}

func (s *Server) HandleBroadcast() func(msg maelstrom.Message) error {
	return func(msg maelstrom.Message) error {
		var body Message
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		s.broadcasts.Add(body.Message)

		return s.node.Reply(msg, map[string]any{
			"type": "broadcast_ok",
		})
	}
}

func (s *Server) HandleRead() func(msg maelstrom.Message) error {
	return func(msg maelstrom.Message) error {
		return s.node.Reply(msg, map[string]any{
			"type":     "read_ok",
			"messages": s.broadcasts.Read(),
		})
	}
}

func (s *Server) HandleTopology() func(msg maelstrom.Message) error {
	return func(msg maelstrom.Message) error {
		var body Message
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		if s.topology.Load() == nil {
			s.topology.CompareAndSwap(nil, &body.Topology)
		}

		return s.node.Reply(msg, map[string]any{
			"type": "topology_ok",
		})
	}
}
