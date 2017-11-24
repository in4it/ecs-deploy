package main

import (
	"github.com/appleboy/gin-jwt"
	"github.com/gin-gonic/gin"
	"net/http"

	"time"
)

// API struct
type API struct {
	authMiddleware *jwt.GinJWTMiddleware
}

// deploy binding from JSON
type Deploy struct {
	Cluster               string                  `json:"cluster" binding:"required"`
	ServicePort           int64                   `json:"servicePort" binding:"required"`
	ServiceProtocol       string                  `json:"serviceProtocol" binding:"required"`
	DesiredCount          int64                   `json:"desiredCount" binding:"required"`
	MinimumHealthyPercent int64                   `json:"minimumHealthyPercent"`
	MaximumPercent        int64                   `json:"maximumPercent"`
	Containers            []*DeployContainer      `json:"containers" binding:"required,dive"`
	HealthCheck           DeployHealthCheck       `json:"healthCheck"`
	RuleConditions        []*DeployRuleConditions `json:"ruleConditions`
}
type DeployContainer struct {
	ContainerName     string    `json:"containerName" binding:"required"`
	ContainerTag      string    `json:"containerTag" binding:"required"`
	ContainerPort     int64     `json:"containerPort"`
	ContainerCommand  []*string `json:"containerCommand"`
	ContainerImage    string    `json:"containerImage`
	ContainerURI      string    `json:"containerURI"`
	Essential         bool      `json:"essential"`
	Memory            int64     `json:"memory"`
	MemoryReservation int64     `json:"memoryReservation"`
	CPU               int64     `json:"cpu"`
	CPUReservation    int64     `json:"cpuReservation"`
}

type DeployHealthCheck struct {
	HealthyThreshold   int64  `json:"healthyThreshold"`
	UnhealthyThreshold int64  `json:"unhealthyThreshold"`
	Path               string `json:"path"`
	Port               string `json:"port"`
	Protocol           string `json:"protocol"`
	Interval           int64  `json:"interval"`
	Matcher            string `json:"matcher"`
	Timeout            int64  `json:"timeout"`
}
type DeployRuleConditions struct {
	Listeners   []string `json:"listeners"`
	PathPattern string   `json:"pathPattern"`
	Hostname    string   `json:"hostname"`
}

func (a *API) launch() {
	a.createAuthMiddleware()
	a.createRoutes()
}

func (a *API) createRoutes() {
	// create
	r := gin.Default()

	// prefix
	prefix := getEnv("URL_PREFIX", "")
	apiPrefix := prefix + getEnv("URL_PREFIX_API", "/api/v1")

	auth := r.Group(apiPrefix)
	auth.Use(a.authMiddleware.MiddlewareFunc())
	{
		// health check
		r.GET(prefix+"/health", a.healthHandler)

		// login handlers
		r.POST(prefix+"/login", a.authMiddleware.LoginHandler)
		// health with auth
		auth.GET("/health", a.healthHandler)

		// refresh token
		auth.GET("/refresh_token", a.authMiddleware.RefreshHandler)

		// ECR
		auth.POST("/ecr/create/:repository", a.ecrCreateHandler)

		// Deploy
		auth.POST("/deploy/:service", a.deployServiceHandler)

		// Export
		auth.GET("/export/:terraform", a.exportTerraformHandler)
	}

	// run API
	r.Run()
}
func (a *API) createAuthMiddleware() {
	a.authMiddleware = &jwt.GinJWTMiddleware{
		Realm:      "ecs-deploy",
		Key:        []byte(getEnv("JWT_SECRET", "unsecure secret key 8a045eb")),
		Timeout:    time.Hour,
		MaxRefresh: time.Hour,
		Authenticator: func(userId string, password string, c *gin.Context) (string, bool) {
			if (userId == "deploy" && password == getEnv("DEPLOY_PASSWORD", "deploy")) || (userId == "developer" && password == getEnv("DEVELOPER_PASSWORD", "developer")) {
				return userId, true
			}

			return userId, false
		},
		Authorizator: func(userId string, c *gin.Context) bool {
			if userId == "deploy" {
				return true
			}

			return false
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},
		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		// - "cookie:<name>"
		TokenLookup: "header:Authorization",
		// TokenLookup: "query:token",
		// TokenLookup: "cookie:token",

		// TokenHeadName is a string in the header. Default value is "Bearer"
		TokenHeadName: "Bearer",

		// TimeFunc provides the current time. You can override it to use another time value. This is useful for testing or if your server uses a different time zone than your tokens.
		TimeFunc: time.Now,
	}
}

func (a *API) ecrCreateHandler(c *gin.Context) {
	controller := Controller{}
	res, err := controller.createRepository(c.Param("repository"))
	if err == nil {
		c.JSON(200, gin.H{
			"message": res,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}

func (a *API) healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "OK",
	})
}
func (a *API) deployServiceHandler(c *gin.Context) {
	var json Deploy
	controller := Controller{}
	if err := c.ShouldBindJSON(&json); err == nil {
		// validate service name
		if len(c.Param("service")) > 2 {
			res, err := controller.deploy(c.Param("service"), json)
			if err == nil {
				c.JSON(200, gin.H{
					"message": res,
				})
			} else {
				c.JSON(200, gin.H{
					"error": err.Error(),
				})
			}
		} else {
			c.JSON(200, gin.H{
				"error": "service name needs to be at least 3 characters",
			})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
func (a *API) exportTerraformHandler(c *gin.Context) {
	e := Export{}
	exp, err := e.terraform()
	if err == nil {
		if exp == nil {
			c.JSON(200, gin.H{
				"export": "",
			})
		} else {
			c.JSON(200, gin.H{
				"export": *exp,
			})
		}
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
