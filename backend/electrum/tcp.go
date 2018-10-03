package electrum

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net"
	"time"
)

var DebugMode bool

const (
	connTimeout  = 2 * time.Second
	writeTimeout = 10 * time.Second
	readTimeout  = 10 * time.Second
	messageDelim = byte('\n')
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrNodeConnected  = errors.New("node already connected")
	ErrNodeShutdown   = errors.New("node has shutdown")
	ErrIdMismatch     = errors.New("response id mismatch")
	ErrUnknown        = errors.New("unknown error")
	ErrNetwork        = errors.New("network error")
	ErrAPI            = errors.New("received API error")
)

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type RequestMessage struct {
	Id     uint64        `json:"id"`
	Method string        `json:"method"`
	Params []interface{} `json:"params"`
}

type ResponseMessage struct {
	Id      uint64         `json:"id"`
	JsonRpc string         `json:"jsonrpc"`
	Result  interface{}    `json:"result"`
	Error   *ErrorResponse `json:"error"`
}

type Transport interface {
	SendMessage(RequestMessage) (*ResponseMessage, error)
	Shutdown() error
}

type TCPTransport struct {
	conn net.Conn
}

func NewTCPTransport(addr string) (Transport, error) {
	conn, err := net.DialTimeout("tcp", addr, connTimeout)
	if err != nil {
		return nil, err
	}

	t := &TCPTransport{conn: conn}

	return t, nil
}

func NewSSLTransport(addr string) (Transport, error) {
	d := &net.Dialer{
		Timeout: connTimeout,
	}

	conn, err := tls.DialWithDialer(d, "tcp", addr, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, err
	}

	t := &TCPTransport{conn: conn}

	return t, nil
}

func (t *TCPTransport) SendMessage(request RequestMessage) (*ResponseMessage, error) {
	if t.conn == nil {
		return nil, ErrNodeShutdown
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	body = append(body, messageDelim)

	// Set write deadline
	_ = t.conn.SetWriteDeadline(time.Now().Add(writeTimeout))

	// Send message
	n, err := t.conn.Write(body)
	if err != nil {
		_ = t.Shutdown()
		if DebugMode {
			log.Printf("error on send to %s: %s", t.conn.RemoteAddr(), err)
		}
		return nil, ErrNetwork
	}
	if n != len(body) {
		_ = t.Shutdown()
		if DebugMode {
			log.Printf("error on send to %s: short write (%d < %d)", t.conn.RemoteAddr(), n, len(body))
		}
		return nil, ErrNetwork
	}

	if DebugMode {
		log.Printf("%s <- %s", t.conn.RemoteAddr(), body)
	}

	// Clear write deadline, set read deadline
	_ = t.conn.SetWriteDeadline(time.Time{})
	_ = t.conn.SetReadDeadline(time.Now().Add(readTimeout))

	// Wait for response
	reader := bufio.NewReader(t.conn)

	line, err := reader.ReadBytes(messageDelim)
	if err != nil {
		_ = t.Shutdown()
		if DebugMode {
			log.Printf("error on recv from %s: %s", t.conn.RemoteAddr(), err)
		}
		return nil, ErrNetwork
	}

	// Clear deadline
	_ = t.conn.SetReadDeadline(time.Time{})

	if DebugMode {
		log.Printf("%s -> %s", t.conn.RemoteAddr(), line)
	}

	// Parse & process message
	resp := ResponseMessage{}
	err = json.Unmarshal(line, &resp)
	if err != nil {
		_ = t.Shutdown()
		if DebugMode {
			log.Printf("error on recv from %s: %s", t.conn.RemoteAddr(), err)
		}
		return nil, ErrUnknown
	}

	if resp.Id != request.Id {
		_ = t.Shutdown()
		if DebugMode {
			log.Printf("error on recv from %s: id mismatch (%d != %d)", t.conn.RemoteAddr(), request.Id, resp.Id)
		}
		return nil, ErrIdMismatch
	}

	if resp.Error != nil {
		if DebugMode {
			log.Printf("error on recv from %s: server error (%d: %s)", t.conn.RemoteAddr(), resp.Error.Code, resp.Error.Message)
		}
		return nil, ErrAPI
	}

	return &resp, nil
}

func (t *TCPTransport) Shutdown() error {
	return t.conn.Close()
}
