package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func testHTTPRequest(h http.Handler, method string, url string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, body)
	h.ServeHTTP(w, req)
	return w
}

func testStartGame(t *testing.T, r http.Handler, query string) string {
	w := testHTTPRequest(r, "GET", "/games/start"+query, nil)

	var startResponse struct {
		ID string
	}
	require.Equal(t, 200, w.Code)
	require.Nil(t, json.Unmarshal(w.Body.Bytes(), &startResponse))
	return startResponse.ID
}

// testReadMessage reads server messages until one of the wanted type
// arrives (skipping broadcasts like PLAYERS we don't care about) and
// unmarshals its data into out.
func testReadMessage(t *testing.T, conn *websocket.Conn, wantType string, out any) {
	require.Nil(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for {
		var msg webSocketMessage
		require.Nil(t, conn.ReadJSON(&msg), "waiting for %s", wantType)
		if msg.Type != wantType {
			continue
		}
		if out != nil {
			require.Nil(t, json.Unmarshal(msg.Data, out))
		}
		return
	}
}

func testSendMessage(t *testing.T, conn *websocket.Conn, msgType string, data any) {
	msg, err := newWebSocketMessage(msgType, data)
	require.Nil(t, err)
	require.Nil(t, conn.WriteJSON(msg))
}

func TestGamesStartJoinLeave(t *testing.T) {
	r := gin.Default()
	gamesAddRoutes(r)

	id := testStartGame(t, r, "")
	conn := testOpenWebSocket(t, r, "/games/"+id)

	testReadMessage(t, conn, "JOINED", nil)

	time.Sleep(500 * time.Millisecond)
	require.Nil(t, webSocketSendClose(conn, ""))
	conn.Close()
	time.Sleep(1 * time.Second)
}

// TestFullSinglePlayerGame plays a whole 2-round game through the
// websocket protocol and checks the scoring on the way.
func TestFullSinglePlayerGame(t *testing.T) {
	r := gin.Default()
	gamesAddRoutes(r)

	id := testStartGame(t, r, "?rounds=2&seconds=30")
	conn := testOpenWebSocket(t, r, "/games/"+id)
	defer conn.Close()

	var joined struct {
		You    string `json:"you"`
		Rounds int    `json:"rounds"`
	}
	testReadMessage(t, conn, "JOINED", &joined)
	require.Equal(t, 2, joined.Rounds)

	testSendMessage(t, conn, "HELLO", map[string]string{"name": "Reggie"})
	testSendMessage(t, conn, "START", nil)

	total := 0
	for round := 1; round <= 2; round++ {
		var start roundStartData
		testReadMessage(t, conn, "ROUND_START", &start)
		require.Equal(t, round, start.Round)
		require.NotEmpty(t, start.Image)
		require.Greater(t, start.EndsAtMs, time.Now().UnixMilli())

		// Guess a fixed spot on the Quad; distance depends on which
		// location was picked, but the score must match the formula.
		guess := playerGuess{Lat: 40.5085, Lng: -88.9917}
		testSendMessage(t, conn, "GUESS", guess)

		var result roundResultData
		testReadMessage(t, conn, "ROUND_RESULT", &result)
		require.Equal(t, round, result.Round)
		require.Len(t, result.Results, 1)

		entry := result.Results[0]
		require.Equal(t, "Reggie", entry.Name)
		require.True(t, entry.Guessed)

		wantDistance := haversineMeters(guess.Lat, guess.Lng, result.Actual.Lat, result.Actual.Lng)
		require.InDelta(t, wantDistance, entry.DistanceMeters, 0.01)
		require.Equal(t, scoreForDistance(wantDistance), entry.Score)

		total += entry.Score
		require.Equal(t, total, entry.Total)

		testSendMessage(t, conn, "NEXT", nil)
	}

	var over struct {
		Standings []playerInfo `json:"standings"`
	}
	testReadMessage(t, conn, "GAME_OVER", &over)
	require.Len(t, over.Standings, 1)
	require.Equal(t, total, over.Standings[0].Total)
}

// TestTwoPlayerRoundResolves checks that a round ends as soon as every
// connected player has guessed, and both players get scored.
func TestTwoPlayerRoundResolves(t *testing.T) {
	r := gin.Default()
	gamesAddRoutes(r)

	id := testStartGame(t, r, "?rounds=1&seconds=30")
	conn1 := testOpenWebSocket(t, r, "/games/"+id)
	defer conn1.Close()
	conn2 := testOpenWebSocket(t, r, "/games/"+id)
	defer conn2.Close()

	testReadMessage(t, conn1, "JOINED", nil)
	testReadMessage(t, conn2, "JOINED", nil)

	testSendMessage(t, conn1, "START", nil)
	testReadMessage(t, conn1, "ROUND_START", nil)
	testReadMessage(t, conn2, "ROUND_START", nil)

	testSendMessage(t, conn1, "GUESS", playerGuess{Lat: 40.509, Lng: -88.991})
	testSendMessage(t, conn2, "GUESS", playerGuess{Lat: 40.507, Lng: -88.989})

	var result roundResultData
	testReadMessage(t, conn1, "ROUND_RESULT", &result)
	require.Len(t, result.Results, 2)
	for _, entry := range result.Results {
		require.True(t, entry.Guessed)
		require.Equal(t, scoreForDistance(entry.DistanceMeters), entry.Score)
	}
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
