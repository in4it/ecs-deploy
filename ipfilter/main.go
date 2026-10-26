package ipfilter

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
)

var whitelistLogger = loggo.GetLogger("whitelist")

// SetTrustedProxies configures which proxies (comma separated IPs or CIDRs) are trusted
// to set X-Forwarded-For. When empty, all proxies stay trusted (gin's default)
func SetTrustedProxies(r *gin.Engine, trustedProxies string) error {
	if strings.TrimSpace(trustedProxies) == "" {
		whitelistLogger.Warningf("ECS_TRUSTED_PROXIES not set: trusting X-Forwarded-For from all clients")
		return nil
	}
	proxies := strings.Split(trustedProxies, ",")
	for i := range proxies {
		proxies[i] = strings.TrimSpace(proxies[i])
	}
	return r.SetTrustedProxies(proxies)
}

// IP filtering handlerfunc
func IPWhiteList(whitelist string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := net.ParseIP(c.ClientIP())
		whitelistLogger.Tracef("Client IP: %s", clientIP)
		whitelistLogger.Tracef("IP whitelist: %s", whitelist)
		if clientIP == nil {
			whitelistLogger.Errorf("Error: Missing or unsupported format in header")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"status":  http.StatusForbidden,
				"message": "Permission denied",
			})
			return
		}
		subnets := strings.Split(whitelist, ",")
		for i := range subnets {
			subnets[i] = strings.TrimSpace(subnets[i])
		}
		for _, s := range subnets {
			_, ipnet, err := net.ParseCIDR(s)
			if err != nil {
				whitelistLogger.Errorf("Malformed whitelist argument: %s", s)
			} else {
				whitelistLogger.Tracef("Whitelist: %s", ipnet)
				whitelistLogger.Tracef("Client: %s", clientIP)
				if ipnet.Contains(clientIP) {
					whitelistLogger.Tracef("Client IP match subnet: %s", ipnet)
					return
				}
			}
		}

		whitelistLogger.Errorf("Blocked access from: %s", clientIP)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"status":  http.StatusForbidden,
			"message": "Permission denied",
		})
		return
	}
}
