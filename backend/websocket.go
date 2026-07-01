package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type webSocketMessage struct {
	Type string          `json:"t"`
	Data json.RawMessage `json:"d,omitempty"`
}

func newWebSocketMessage(t string, d any) (*webSocketMessage, error) {
	j, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	msg := &webSocketMessage{
		Type: t,
		Data: j,
	}
	return msg, nil
}

func newWebSocketMessageAssert(t string, d any) *webSocketMessage {
	if msg, err := newWebSocketMessage(t, d); err != nil {
		panic(err)
	} else {
		return msg
	}
}

type webSocketConnection struct {
	conn *websocket.Conn
	Tx   chan *webSocketMessage
	Rx   chan *webSocketMessage
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (c *webSocketConnection) Close() {
	close(c.Tx)
}

func (c *webSocketConnection) loop() {
	defer c.conn.Close()

	done := make(chan struct{})
	go func() {
		defer close(c.Rx)
		for {
			var rxMsg webSocketMessage
			if err := c.conn.ReadJSON(&rxMsg); err != nil {
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
				webSocketSendClose(c.conn, "")
				break out
			}

			if err := c.conn.WriteJSON(txMsg); err != nil {
				log.Printf("WebSocket write failure: %v", err)
				break out
			}
		}
	}
}

func openWebSocket(c *gin.Context) (*webSocketConnection, error) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, err
	}

	wrapper := &webSocketConnection{
		conn: conn,
		Tx:   make(chan *webSocketMessage),
		Rx:   make(chan *webSocketMessage),
	}
	go wrapper.loop()
	return wrapper, nil
}

func webSocketSendClose(c *websocket.Conn, text string) error {
	return c.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, text),
		time.Now().Add(5*time.Second))
}
