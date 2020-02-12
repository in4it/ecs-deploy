package ipfilter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupMiddleware(whitelist string) *gin.Engine {
	r := gin.Default()
	r.Use(IPWhiteList(whitelist))
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})
	return r
}

func TestPingRoute(t *testing.T) {
	router := setupMiddleware("0.0.0.0/0")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.Header.Add("X-Forwarded-For", "127.0.0.1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}

func TestProcessingInput(t *testing.T) {
	router := setupMiddleware("10.10.10.0/24,20.20.20.0/24,30.30.30.0/24,0.0.0.0/0")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.Header.Add("X-Forwarded-For", "20.20.20.5")
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}

func TestDenyRoute(t *testing.T) {
	router := setupMiddleware("10.10.10.0/24")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.Header.Add("X-Forwarded-For", "127.0.0.1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	assert.Equal(t, "{\"message\":\"Permission denied\",\"status\":403}", w.Body.String())
}

func TestBadClientIPRoute(t *testing.T) {
	router := setupMiddleware("10.10.10.0/24")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.Header.Add("X-Forwarded-For", "10.10.10.1badinput")
	router.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	assert.Equal(t, "{\"message\":\"Permission denied\",\"status\":403}", w.Body.String())
}

func TestBadInput(t *testing.T) {
	router := setupMiddleware("0.0.0.0/0badinput")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.Header.Add("X-Forwarded-For", "127.0.0.1")
	router.ServeHTTP(w, req)

	assert.Equal(t, 403, w.Code)
	assert.Equal(t, "{\"message\":\"Permission denied\",\"status\":403}", w.Body.String())
}
