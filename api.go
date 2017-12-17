package main

import (
	//"github.com/RobotsAndPencils/go-saml"
	"github.com/appleboy/gin-jwt"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/in4it/ecs-deploy/docs"
	"github.com/swaggo/gin-swagger"              // gin-swagger middleware
	"github.com/swaggo/gin-swagger/swaggerFiles" // swagger embed files

	"errors"
	//"io/ioutil"
	"net/http"
	"time"
)

// API struct
type API struct {
	authMiddleware *jwt.GinJWTMiddleware
	//sp             saml.ServiceProviderSettings
	samlHelper *SAML
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

type DeployResult struct {
	ServiceName       string    `json:"serviceName"`
	ClusterName       string    `json:"clusterName"`
	TaskDefinitionArn string    `json:"taskDefinitionArn"`
	Status            string    `json:"status"`
	DeploymentTime    time.Time `json:"deploymentTime"`
}

type RunningService struct {
	ServiceName  string                     `json:"serviceName"`
	ClusterName  string                     `json:"clusterName"`
	RunningCount int64                      `json:"runningCount"`
	Status       string                     `json:"status"`
	Deployments  []RunningServiceDeployment `json:"deployments"`
}
type RunningServiceDeployment struct {
	Status       string    `json:"status"`
	RunningCount int64     `json:"runningCount"`
	PendingCount int64     `json:"pendingCount"`
	DesiredCount int64     `json:"desiredCount"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (a *API) launch() error {
	if getEnv("SAML_ENABLED", "") == "yes" {
		err := a.initSAML()
		if err != nil {
			return err
		}
	}

	a.createAuthMiddleware()
	a.createRoutes()

	return nil
}

func (a *API) initSAML() error {
	// initialize samlHelper
	var err error
	a.samlHelper, err = newSAML(getEnv("SAML_METADATA_URL", ""), []byte(getEnv("SAML_CERTIFICATE", "")), []byte(getEnv("SAML_PRIVATE_KEY", "")))
	if err != nil {
		return err
	}

	return nil
}

func (a *API) createRoutes() {
	// create
	r := gin.Default()

	// location
	r.Use(location.Default())

	// prefix
	prefix := getEnv("URL_PREFIX", "")
	apiPrefix := prefix + getEnv("URL_PREFIX_API", "/api/v1")

	// webapp
	r.Static(prefix+"/webapp", "./webapp/dist")

	auth := r.Group(apiPrefix)
	auth.Use(a.authMiddleware.MiddlewareFunc())
	{
		// frontend redirect
		r.GET(prefix, a.redirectFrontendHandler)
		r.GET(prefix+"/", a.redirectFrontendHandler)

		// health check
		r.GET(prefix+"/health", a.healthHandler)

		// saml init
		if getEnv("SAML_ENABLED", "") == "yes" {
			r.POST(prefix+"/saml/acs", a.samlHelper.samlInitHandler)
			r.GET(prefix+"/saml/acs", a.samlHelper.samlInitHandler)
		}
		r.GET(prefix+"/saml/enabled", a.samlHelper.samlEnabledHandler)

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
		auth.GET("/export/terraform/:service/listenerrulearn", a.exportTerraformListenerRuleArnsHandler)
		auth.GET("/export/terraform/:service/listenerrulearn/:rule", a.exportTerraformListenerRuleArnHandler)

		// deploy list
		auth.GET("/deploy/list", a.listDeploysHandler)
		auth.GET("/deploy/list/:service", a.listDeploysForServiceHandler)
		auth.GET("/deploy/status/:service/:time", a.getServiceStatusHandler)
		// service list
		auth.GET("/service/list", a.listServicesHandler)
		// service list
		auth.GET("/service/describe", a.describeServicesHandler)
		// get service information
		auth.GET("/service/describe/:service", a.describeServiceHandler)

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
		Realm:            "ecs-deploy",
		Key:              []byte(getEnv("JWT_SECRET", "unsecure secret key 8a045eb")),
		SigningAlgorithm: "HS256",
		Timeout:          time.Hour,
		MaxRefresh:       time.Hour,
		Authenticator: func(userId string, password string, c *gin.Context) (string, bool) {
			if (userId == "deploy" && password == getEnv("DEPLOY_PASSWORD", "deploy")) || (userId == "developer" && password == getEnv("DEVELOPER_PASSWORD", "developer")) {
				return userId, true
			}

			return userId, false
		},
		Authorizator: func(userId string, c *gin.Context) bool {
			if userId != "" {
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
		if err = a.deployServiceValidator(c.Param("service"), json); err == nil {
			res, err := controller.deploy(c.Param("service"), json)
			if err == nil {
				c.JSON(200, gin.H{
					"message": res,
				})
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			}
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func (a *API) deployServiceValidator(serviceName string, d Deploy) error {
	if len(serviceName) < 3 {
		return errors.New("service name needs to be at least 3 characters")
	}
	t := false
	for _, container := range d.Containers {
		if container.ContainerName == serviceName {
			t = true
		}
	}
	if !t {
		return errors.New("At least one container needs to have the same name as the service (" + serviceName + ")")
	}

	return nil
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

// @summary Export listener rule arn
// @description Export listener rule arn
// @id export-terraform-listener-rule-arn
// @produce  json
// @router /api/v1/export/terraform/:service/listenerrulearn/:rule [get]
func (a *API) exportTerraformListenerRuleArnHandler(c *gin.Context) {
	e := Export{}
	listenerRuleArn, err := e.getListenerRuleArn(c.Param("service"), c.Param("rule"))
	if err == nil && listenerRuleArn != nil {
		c.JSON(200, gin.H{
			"listenerRuleArn": listenerRuleArn,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}

// @summary Export listener rule arns
// @description Export listener rule arns
// @id export-terraform-listener-rule-arns
// @produce  json
// @router /api/v1/export/terraform/:service/listenerrulearn [get]
func (a *API) exportTerraformListenerRuleArnsHandler(c *gin.Context) {
	e := Export{}
	listenerRuleArns, err := e.getListenerRuleArns(c.Param("service"))
	if err == nil && listenerRuleArns != nil {
		c.JSON(200, gin.H{
			"listenerRuleKeys": listenerRuleArns.RuleKeys,
			"listenerRules":    listenerRuleArns.Rules,
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

func (a *API) describeServicesHandler(c *gin.Context) {
	controller := Controller{}
	services, err := controller.describeServices()
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
func (a *API) describeServiceHandler(c *gin.Context) {
	controller := Controller{}
	service, err := controller.describeService(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"service": service,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) getServiceStatusHandler(c *gin.Context) {
	controller := Controller{}
	service, err := controller.getDeploymentStatus(c.Param("service"), c.Param("time"))
	if err == nil {
		c.JSON(200, gin.H{
			"service": service,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}

func (a *API) redirectFrontendHandler(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, getEnv("URL_PREFIX", "")+"/webapp/")
}
