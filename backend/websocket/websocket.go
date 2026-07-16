package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/posener/wstest"
	"github.com/stretchr/testify/require"
)

type Message struct {
	Type string          `json:"t"`
	Data json.RawMessage `json:"d,omitempty"`
}

func NewMessage(t string, d any) (*Message, error) {
	j, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Type: t,
		Data: j,
	}
	return msg, nil
}

func NewMessageAssert(t string, d any) *Message {
	if msg, err := NewMessage(t, d); err != nil {
		panic(err)
	} else {
		return msg
	}
}

type Connection struct {
	Conn *websocket.Conn
	Tx   chan *Message
	Rx   chan *Message
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (c *Connection) Close() {
	close(c.Tx)
}

func (c *Connection) loop() {
	defer c.Conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(c.Rx)
		for {
			var rxMsg Message
			if err := c.Conn.ReadJSON(&rxMsg); err != nil {
				log.Printf("WebSocket read failure: %v", err)
				close(done)
				break
			} else {
				c.Rx <- &rxMsg
			}
		}
	}()

out:
	for {
		select {
		case <-done:
			break out
		case txMsg, more := <-c.Tx:
			if !more {
				SendClose(c.Conn, "")
				break out
			}

			if err := c.Conn.WriteJSON(txMsg); err != nil {
				log.Printf("WebSocket write failure: %v", err)
				break out
			}
		}
	}
}

func Open(c *gin.Context) (*Connection, error) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, err
	}

	wrapper := &Connection{
		Conn: conn,
		Tx:   make(chan *Message),
		Rx:   make(chan *Message),
	}
	go wrapper.loop()
	return wrapper, nil
}

func OpenTest(t *testing.T, h http.Handler, url string) *websocket.Conn {
	dialer := wstest.NewDialer(h)
	conn, resp, err := dialer.Dial("ws://test.net"+url, nil)
	require.Nil(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	return conn
}

func SendClose(c *websocket.Conn, text string) error {
	return c.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, text),
		time.Now().Add(5*time.Second))
}
