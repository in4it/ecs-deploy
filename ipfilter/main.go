package ipfilter

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/juju/loggo"
)

var whitelistLogger = loggo.GetLogger("whitelist")

// IP filtering handlerfunc
func IPWhiteList(whitelist string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := net.ParseIP(c.ClientIP())
		whitelistLogger.Debugf("Client IP: %s", clientIP)
		whitelistLogger.Debugf("IP whitelist: %s", whitelist)
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
				whitelistLogger.Debugf("Whitelist: %s", ipnet)
				whitelistLogger.Debugf("Client: %s", clientIP)
				if ipnet.Contains(clientIP) {
					whitelistLogger.Debugf("Client IP match subnet: %s", ipnet)
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
