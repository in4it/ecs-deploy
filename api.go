package main

import (
	"github.com/appleboy/gin-jwt"
	"github.com/gin-gonic/gin"
	_ "github.com/in4it/ecs-deploy/docs"
	"github.com/swaggo/gin-swagger"              // gin-swagger middleware
	"github.com/swaggo/gin-swagger/swaggerFiles" // swagger embed files

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

		// swagger
		r.GET(prefix+"/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
		auth.GET("/export/terraform", a.exportTerraformHandler)
		auth.GET("/export/terraform/:service/targetgrouparn", a.exportTerraformTargetGroupArnHandler)

		// deploy list
		auth.GET("/deploy/list", a.listDeploysHandler)
		auth.GET("/deploy/list/:service", a.listDeploysForServiceHandler)
		// service list
		auth.GET("/service/list", a.listServicesHandler)
		// get service information
		//auth.GET("/service/:service", a.getServiceHandler)
	}

	// run API
	r.Run()
}

// @summary login to receive jwt token
// @description login with user and password to receive jwt token
// @id login
// @accept  json
// @produce  json
// @router /login [post]
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

// @summary Create ECR repository
// @description Creates AWS ECR (Docker) repository using repository name as parameter
// @id ecr-create-repository
// @accept  json
// @produce  json
// @param   repository     path    string     true        "repository"
// @router /api/v1/ecr/create/{repository} [post]
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

// @summary Healthcheck
// @description Healthcheck for loadbalancer
// @id healthcheck
// @accept  json
// @produce  json
// @router /api/v1/healthcheck [get]
func (a *API) healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "OK",
	})
}

// @summary Deploy service to ECS
// @description Deploy a service to ECS
// @id ecs-deploy-service
// @accept  json
// @produce  json
// @param   service         path    string     true        "service name"
// @router /api/v1/deploy/{service} [post]
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

// @summary Export current services to terraform
// @description Export service data stored in dynamodb into terraform tf files
// @id export-terraform
// @produce  json
// @router /api/v1/export/terraform [get]
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

// @summary Export targetgroup arn
// @description Export target group arn stored in dynamodb into terraform tf files
// @id export-terraform-targetgroup-arn
// @produce  json
// @router /api/v1/export/terraform/:service/targetgrouparn [get]
func (a *API) exportTerraformTargetGroupArnHandler(c *gin.Context) {
	e := Export{}
	targetGroupArn, err := e.getTargetGroupArn(c.Param("service"))
	if err == nil && targetGroupArn != nil {
		c.JSON(200, gin.H{
			"targetGroupArn": targetGroupArn,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) listDeploysHandler(c *gin.Context) {
	controller := Controller{}
	deploys, err := controller.getDeploys()
	if err == nil {
		c.JSON(200, gin.H{
			"deployments": deploys,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) listDeploysForServiceHandler(c *gin.Context) {
	controller := Controller{}
	deploys, err := controller.getDeploysForService(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"deployments": deploys,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) listServicesHandler(c *gin.Context) {
	controller := Controller{}
	services, err := controller.getServices()
	if err == nil {
		c.JSON(200, gin.H{
			"services": services,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
