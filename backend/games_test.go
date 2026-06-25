package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func testGinRequest(r *gin.Engine, method string, url string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, body)
	r.ServeHTTP(w, req)
	return w
}

func TestGamesStartJoin(t *testing.T) {
	r := gin.Default()
	gamesAddRoutes(r)

	w := testGinRequest(r, "GET", "/games/start", nil)

	var startResponse struct {
		ID string
	}

	require.Equal(t, 200, w.Code)
	require.Nil(t, json.Unmarshal(w.Body.Bytes(), &startResponse))

	w = testGinRequest(r, "GET", "/games/"+startResponse.ID, nil)

	require.Equal(t, 400, w.Code)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}
