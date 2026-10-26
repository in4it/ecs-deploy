package ipfilter

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupMiddleware(t *testing.T, whitelist, trustedProxies string) *gin.Engine {
	r := gin.Default()
	assert.NoError(t, SetTrustedProxies(r, trustedProxies))
	r.Use(IPWhiteList(whitelist))
	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})
	return r
}

func doRequest(router *gin.Engine, remoteAddr, forwardedFor string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Add("X-Forwarded-For", forwardedFor)
	}
	router.ServeHTTP(w, req)
	return w
}

func assertDenied(t *testing.T, w *httptest.ResponseRecorder) {
	assert.Equal(t, 403, w.Code)
	assert.Equal(t, "{\"message\":\"Permission denied\",\"status\":403}", w.Body.String())
}

func TestPingRoute(t *testing.T) {
	router := setupMiddleware(t, "0.0.0.0/0", "")
	w := doRequest(router, "192.0.2.1:1234", "127.0.0.1")

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}

func TestProcessingInput(t *testing.T) {
	router := setupMiddleware(t, "10.10.10.0/24,20.20.20.0/24,30.30.30.0/24,0.0.0.0/0", "")
	w := doRequest(router, "192.0.2.1:1234", "20.20.20.5")

	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "pong", w.Body.String())
}

func TestDenyRoute(t *testing.T) {
	router := setupMiddleware(t, "10.10.10.0/24", "")
	assertDenied(t, doRequest(router, "192.0.2.1:1234", "127.0.0.1"))
}

func TestBadClientIPRoute(t *testing.T) {
	router := setupMiddleware(t, "10.10.10.0/24", "")
	assertDenied(t, doRequest(router, "", "10.10.10.1badinput"))
}

func TestBadInput(t *testing.T) {
	router := setupMiddleware(t, "0.0.0.0/0badinput", "")
	assertDenied(t, doRequest(router, "192.0.2.1:1234", "127.0.0.1"))
}

func TestNoTrustedProxiesTrustsForwardedFor(t *testing.T) {
	// default behavior: X-Forwarded-For is trusted from every client
	router := setupMiddleware(t, "10.10.10.0/24", "")
	w := doRequest(router, "203.0.113.9:1234", "10.10.10.5")

	assert.Equal(t, 200, w.Code)
}

func TestSpoofedForwardedForIgnored(t *testing.T) {
	// client isn't a trusted proxy, so its X-Forwarded-For header is ignored
	router := setupMiddleware(t, "10.10.10.0/24", "172.16.0.0/12")
	assertDenied(t, doRequest(router, "203.0.113.9:1234", "10.10.10.5"))
}

func TestForwardedForFromTrustedProxy(t *testing.T) {
	router := setupMiddleware(t, "10.10.10.0/24", "172.16.0.0/12, 192.168.1.1")
	w := doRequest(router, "172.16.5.5:1234", "10.10.10.5")
	assert.Equal(t, 200, w.Code)

	w = doRequest(router, "192.168.1.1:1234", "10.10.10.6")
	assert.Equal(t, 200, w.Code)

	// a trusted proxy forwarding a client outside the whitelist
	assertDenied(t, doRequest(router, "172.16.5.5:1234", "203.0.113.9"))
}

func TestDirectClientWithTrustedProxies(t *testing.T) {
	router := setupMiddleware(t, "10.10.10.0/24", "172.16.0.0/12")
	w := doRequest(router, "10.10.10.7:1234", "")

	assert.Equal(t, 200, w.Code)
}

func TestInvalidTrustedProxies(t *testing.T) {
	r := gin.New()
	assert.Error(t, SetTrustedProxies(r, "172.16.0.0/12,notanip"))
}
