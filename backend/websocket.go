package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type webSocketConnection struct {
	conn *websocket.Conn
	msg  chan any
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func webSocketLoop(wrapper *webSocketConnection) {
	defer wrapper.conn.Close()
	defer close(wrapper.msg)

	done := make(chan struct{})
	go func() {
		for {
			var rxMsg map[string]any
			err := wrapper.conn.ReadJSON(&rxMsg)
			if err != nil {
				log.Printf("WebSocket read failure: %v", err)
				close(done)
				break
			} else {
				wrapper.msg <- rxMsg
			}
		}
	}()

out:
	for {
		select {
		case <-done:
			break out
		case txMsg := <-wrapper.msg:
			if err := wrapper.conn.WriteJSON(txMsg); err != nil {
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
		msg:  make(chan any),
	}
	go webSocketLoop(wrapper)
	return wrapper, nil
}
