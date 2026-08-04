package games

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"isu-geoguesser/database"
	ws "isu-geoguesser/websocket"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func testHTTPRequest(h http.Handler, method string, url string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, body)
	h.ServeHTTP(w, req)
	return w
}

func TestGamesStartJoinLeave(t *testing.T) {
	database.Open()

	r := gin.Default()
	AddRoutes(r)

	w := testHTTPRequest(r, "GET", "/games/start", nil)

	var startResponse struct {
		ID string
	}

	require.Equal(t, 200, w.Code)
	require.Nil(t, json.Unmarshal(w.Body.Bytes(), &startResponse))

	conn := ws.OpenTest(t, r, "/games/"+startResponse.ID)

	var msg ws.Message
	require.Nil(t, conn.ReadJSON(&msg))
	require.Equal(t, msg.Type, "JOINED")

	time.Sleep(500 * time.Millisecond)
	require.Nil(t, ws.SendClose(conn, ""))
	conn.Close()
	time.Sleep(1 * time.Second)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
