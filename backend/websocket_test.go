package main

import (
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/posener/wstest"
	"github.com/stretchr/testify/require"
)

func testOpenWebSocket(t *testing.T, h http.Handler, url string) *websocket.Conn {
	dialer := wstest.NewDialer(h)
	conn, resp, err := dialer.Dial("ws://test.net"+url, nil)
	require.Nil(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	return conn
}
