package main

import (
	"encoding/json"
	"log"

	maelstrom "github.com/jepsen-io/maelstrom/demo/go"
)

func main() {
	n := maelstrom.NewNode()

	s := Server{
		node:   n,
		unique: make(map[int]struct{}),
	}

	n.Handle("broadcast", s.HandleBroadcast())
	n.Handle("read", s.HandleRead())
	n.Handle("topology", s.HandleTopology())

	if err := n.Run(); err != nil {
		log.Fatal(err)
	}
}

type Server struct {
	node       *maelstrom.Node
	unique     map[int]struct{}
	broadcasts []int
}

type BroadcastMessage struct {
	Message int `json:"message"`
}

func (s *Server) HandleBroadcast() func(msg maelstrom.Message) error {
	return func(msg maelstrom.Message) error {
		var body BroadcastMessage
		if err := json.Unmarshal(msg.Body, &body); err != nil {
			return err
		}

		if _, ok := s.unique[body.Message]; !ok {
			s.unique[body.Message] = struct{}{}
			s.broadcasts = append(s.broadcasts, body.Message)
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
			"messages": s.broadcasts,
		})
	}
}

func (s *Server) HandleTopology() func(msg maelstrom.Message) error {
	return func(msg maelstrom.Message) error {
		return s.node.Reply(msg, map[string]any{
			"type": "topology_ok",
		})
	}
}
