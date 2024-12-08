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
	neighbors  atomic.Pointer[[]string]
}

func NewServer(node *maelstrom.Node) *Server {
	server := &Server{
		node: node,
		broadcasts: Broadcasts{
			broadcasts: make(map[int]struct{}),
			cached:     make([]int, 0),
		},
	}
	return server
}

type Broadcasts struct {
	mu         sync.RWMutex
	broadcasts map[int]struct{}
	cached     []int
}

func (b *Broadcasts) Add(message int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.broadcasts[message]; !ok {
		b.broadcasts[message] = struct{}{}
		b.cached = append(b.cached, message)
		return true
	}
	return false
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

		if ok := s.broadcasts.Add(body.Message); !ok {
			return s.node.Reply(msg, map[string]any{
				"type": "broadcast_ok",
			})
		}

		neighbors := s.neighbors.Load()
		if neighbors != nil {
			payload := Message{
				Type:    "broadcast",
				Message: body.Message,
			}

			for _, neighbor := range *neighbors {
				if neighbor != msg.Src {
					if err := s.node.Send(neighbor, payload); err != nil {
						return err
					}
				}
			}
		}

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

		if s.neighbors.Load() == nil {
			neighbors := body.Topology[s.node.ID()]
			s.neighbors.CompareAndSwap(nil, &neighbors)
		}

		return s.node.Reply(msg, map[string]any{
			"type": "topology_ok",
		})
	}
}
