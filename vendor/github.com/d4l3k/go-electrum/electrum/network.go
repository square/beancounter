package electrum

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
)

const (
	ClientVersion   = "0.0.1"
	ProtocolVersion = "1.0"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrNodeConnected  = errors.New("node already connected")
)

type Transport interface {
	SendMessage([]byte) error
	Responses() <-chan []byte
	Errors() <-chan error
}

type respMetadata struct {
	Id     int    `json:"id"`
	Method string `json:"method"`
	Error  string `json:"error"`
}

type request struct {
	Id     int      `json:"id"`
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type basicResp struct {
	Result string `json:"result"`
}

type Node struct {
	Address string

	transport    Transport
	handlers     map[int]chan []byte
	handlersLock sync.RWMutex

	pushHandlers     map[string][]chan []byte
	pushHandlersLock sync.RWMutex

	nextId int
}

// NewNode creates a new node.
func NewNode() *Node {
	n := &Node{
		handlers:     make(map[int]chan []byte),
		pushHandlers: make(map[string][]chan []byte),
	}
	return n
}

// ConnectTCP creates a new TCP connection to the specified address.
func (n *Node) ConnectTCP(addr string) error {
	if n.transport != nil {
		return ErrNodeConnected
	}
	n.Address = addr
	transport, err := NewTCPTransport(addr)
	if err != nil {
		return err
	}
	n.transport = transport
	go n.listen()
	return nil
}

// ConnectSLL creates a new SLL connection to the specified address.
func (n *Node) ConnectSSL(addr string, config *tls.Config) error {
	if n.transport != nil {
		return ErrNodeConnected
	}
	n.Address = addr
	transport, err := NewSSLTransport(addr, config)
	if err != nil {
		return err
	}
	n.transport = transport
	go n.listen()
	return nil
}

// err handles errors produced by the foreign node.
func (n *Node) err(err error) {
	// TODO (d4l3k) Better error handling.
	log.Fatal(err)
}

// listen processes messages from the server.
func (n *Node) listen() {
	for {
		select {
		case err := <-n.transport.Errors():
			n.err(err)
			return
		case bytes := <-n.transport.Responses():
			msg := &respMetadata{}
			if err := json.Unmarshal(bytes, msg); err != nil {
				n.err(err)
				return
			}
			if len(msg.Error) > 0 {
				n.err(fmt.Errorf("error from server: %#v", msg.Error))
				return
			}
			if len(msg.Method) > 0 {
				n.pushHandlersLock.RLock()
				handlers := n.pushHandlers[msg.Method]
				n.pushHandlersLock.RUnlock()

				for _, handler := range handlers {
					select {
					case handler <- bytes:
					default:
					}
				}
			}

			n.handlersLock.RLock()
			c, ok := n.handlers[msg.Id]
			n.handlersLock.RUnlock()

			if ok {
				c <- bytes
			}
		}
	}
}

// listenPush returns a channel of messages matching the method.
func (n *Node) listenPush(method string) <-chan []byte {
	c := make(chan []byte, 1)
	n.pushHandlersLock.Lock()
	defer n.pushHandlersLock.Unlock()
	n.pushHandlers[method] = append(n.pushHandlers[method], c)
	return c
}

// request makes a request to the server and unmarshals the response into v.
func (n *Node) request(method string, params []string, v interface{}) error {
	msg := request{
		Id:     n.nextId,
		Method: method,
		Params: params,
	}
	n.nextId++
	bytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	bytes = append(bytes, delim)
	if err := n.transport.SendMessage(bytes); err != nil {
		return err
	}

	c := make(chan []byte, 1)

	n.handlersLock.Lock()
	n.handlers[msg.Id] = c
	n.handlersLock.Unlock()

	resp := <-c

	n.handlersLock.Lock()
	defer n.handlersLock.Unlock()
	delete(n.handlers, msg.Id)

	if err := json.Unmarshal(resp, v); err != nil {
		return nil
	}
	return nil
}
