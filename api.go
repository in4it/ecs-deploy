package ecsdeploy

import (
	//"github.com/RobotsAndPencils/go-saml"
	"github.com/appleboy/gin-jwt"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/in4it/ecs-deploy/docs"
	"github.com/in4it/ecs-deploy/ngserve"
	"github.com/in4it/ecs-deploy/session"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
	"github.com/robbiet480/go.sns"
	"github.com/swaggo/gin-swagger"              // gin-swagger middleware
	"github.com/swaggo/gin-swagger/swaggerFiles" // swagger embed files

	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// logging
var apiLogger = loggo.GetLogger("api")

// version
var apiVersion = "1.2"

// API struct
type API struct {
	authMiddleware *jwt.GinJWTMiddleware
	//sp             saml.ServiceProviderSettings
	samlHelper *SAML
}

// deploy binding from JSON
type DeployServices struct {
	Services []Deploy `json:"services" binding:"required"`
}
type Deploy struct {
	Cluster               string                      `json:"cluster" binding:"required"`
	ServiceName           string                      `json:"serviceName"`
	ServicePort           int64                       `json:"servicePort"`
	ServiceProtocol       string                      `json:"serviceProtocol" binding:"required"`
	DesiredCount          int64                       `json:"desiredCount" binding:"required"`
	MinimumHealthyPercent int64                       `json:"minimumHealthyPercent"`
	MaximumPercent        int64                       `json:"maximumPercent"`
	Containers            []*DeployContainer          `json:"containers" binding:"required,dive"`
	HealthCheck           DeployHealthCheck           `json:"healthCheck"`
	RuleConditions        []*DeployRuleConditions     `json:"ruleConditions`
	NetworkMode           string                      `json:"networkMode"`
	NetworkConfiguration  DeployNetworkConfiguration  `json:"networkConfiguration"`
	PlacementConstraints  []DeployPlacementConstraint `json:"placementConstraints"`
	LaunchType            string                      `json:"launchType"`
	DeregistrationDelay   int64                       `json:"deregistrationDelay"`
	Stickiness            DeployStickiness            `json:"stickiness"`
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
type DeployNetworkConfiguration struct {
	AssignPublicIp string   `json:"assignPublicIp"`
	SecurityGroups []string `json:"securityGroups"`
	Subnets        []string `json:"subnets"`
}
type DeployPlacementConstraint struct {
	Expression string `json:"expression"`
	Type       string `json:"type"`
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
	GracePeriodSeconds int64  `json:"gracePeriodSeconds"`
}
type DeployRuleConditions struct {
	Listeners   []string `json:"listeners"`
	PathPattern string   `json:"pathPattern"`
	Hostname    string   `json:"hostname"`
}
type DeployStickiness struct {
	Enabled  bool  `json:"enabled"`
	Duration int64 `json:"duration"`
}

type DeployResult struct {
	ServiceName       string    `json:"serviceName"`
	ClusterName       string    `json:"clusterName"`
	TaskDefinitionArn string    `json:"taskDefinitionArn"`
	Status            string    `json:"status"`
	DeployError       string    `json:"deployError"`
	DeploymentTime    time.Time `json:"deploymentTime"`
}
type DeployServiceParameter struct {
	Name      string `json:"name" binding:"required"`
	Value     string `json:"value" binding:"required"`
	Encrypted bool   `json:"encrypted"`
}

type RunningService struct {
	ServiceName  string                     `json:"serviceName"`
	ClusterName  string                     `json:"clusterName"`
	RunningCount int64                      `json:"runningCount"`
	PendingCount int64                      `json:"pendingCount"`
	DesiredCount int64                      `json:"desiredCount"`
	Status       string                     `json:"status"`
	Events       []RunningServiceEvent      `json:"events"`
	Deployments  []RunningServiceDeployment `json:"deployments"`
	Tasks        []RunningTask              `json:"tasks"`
}
type RunningServiceDeployment struct {
	Status         string    `json:"status"`
	RunningCount   int64     `json:"runningCount"`
	PendingCount   int64     `json:"pendingCount"`
	DesiredCount   int64     `json:"desiredCount"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	TaskDefinition string    `json:"taskDefinition"`
}
type RunningServiceEvent struct {
	CreatedAt time.Time `json:"createdAt"`
	Id        string    `json:"id"`
	Message   string    `json:"message"`
}
type ServiceVersion struct {
	ImageName  string    `json:"imageName"`
	Tag        string    `json:"tag"`
	ImageId    string    `json:"imageId"`
	LastDeploy time.Time `json:"lastDeploy"`
}
type RunningTask struct {
	ContainerInstanceArn string                 `json:"containerInstanceArn"`
	Containers           []RunningTaskContainer `json:"containers"`
	Cpu                  string                 `json:"cpu"`
	CreatedAt            time.Time              `json:"createdAt"`
	DesiredStatus        string                 `json:"desiredStatus"`
	ExecutionStoppedAt   time.Time              `json:"executionStoppedAt"`
	Group                string                 `json:"group"`
	LastStatus           string                 `json:"lastStatus"`
	LaunchType           string                 `json:"launchType"`
	Memory               string                 `json:"memory"`
	PullStartedAt        time.Time              `json:"pullStartedAt"`
	PullStoppedAt        time.Time              `json:"pullStoppedAt"`
	StartedAt            time.Time              `json:"startedAt"`
	StartedBy            string                 `json:"startedBy"`
	StoppedAt            time.Time              `json:"stoppedAt"`
	StoppedReason        string                 `json:"stoppedReason"`
	StoppingAt           time.Time              `json:"stoppingAt"`
	TaskArn              string                 `json:"taskArn"`
	TaskDefinitionArn    string                 `json:"taskDefinitionArn"`
	Version              int64                  `json:"version"`
}
type RunningTaskContainer struct {
	ContainerArn string `json:"containerArn"`
	ExitCode     int64  `json:"exitCode"`
	LastStatus   string `json:"lastStatus"`
	Name         string `json:"name"`
	Reason       string `json:"reason"`
}

// "Run ad-hoc task" type
type RunTask struct {
	StartedBy          string                     `json:"startedBy"`
	ContainerOverrides []RunTaskContainerOverride `json:"containerOverrides"`
}
type RunTaskContainerOverride struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
}

// SNS payload
type SNSPayload struct {
	Message          string `json:"Message"`
	MessageId        string `json:"MessageId"`
	Signature        string `json:"Signature"`
	SignatureVersion string `json:"SignatureVersion"`
	SigningCertURL   string `json:"SigningCertURL"`
	SubscribeURL     string `json:"SubscribeURL"`
	Subject          string `json:"Subject"`
	Timestamp        string `json:"Timestamp"`
	Token            string `json:"Token"`
	TopicArn         string `json:"TopicArn"`
	Type             string `json:"Type" binding:"required"`
	UnsubscribeURL   string `json:"UnsubscribeURL"`
}

// generic payload (to check detail type)
type SNSPayloadGeneric struct {
	Version    string `json:"version"`
	Id         string `json:"id"`
	DetailType string `json:"detail-type" binding:"required"`
}

// ECS SNS Event
type SNSPayloadEcs struct {
	Version    string              `json:"version"`
	Id         string              `json:"id"`
	DetailType string              `json:"detail-type" binding:"required"`
	Source     string              `json:"source"`
	Account    string              `json:"account"`
	Time       string              `json:"time"`
	Region     string              `json:"region"`
	Resources  []string            `json:"resources"`
	Detail     SNSPayloadEcsDetail `json:"detail"`
}
type SNSPayloadEcsDetail struct {
	ClusterArn           string                      `json:"clusterArn"`
	ContainerInstanceArn string                      `json:"containerInstanceArn"`
	Ec2InstanceId        string                      `json:"ec2InstanceId"`
	RegisteredResources  []ContainerInstanceResource `json:"registeredResources"`
	RemainingResources   []ContainerInstanceResource `json:"remainingResources"`
	Status               string                      `json:"status"`
	Version              int64                       `json:"version"`
	VersionInfo          EcsVersionInfo              `json:"versionInfo"`
	UpdatedAt            string                      `json:"updatedAt"`
	RegisteredAt         string                      `json:"registeredAt"`
}

// lifecycle event
type SNSPayloadLifecycle struct {
	Version    string                    `json:"version"`
	Id         string                    `json:"id"`
	DetailType string                    `json:"detail-type" binding:"required"`
	Source     string                    `json:"source"`
	Account    string                    `json:"account"`
	Time       string                    `json:"time"`
	Region     string                    `json:"region"`
	Resources  []string                  `json:"resources"`
	Detail     SNSPayloadLifecycleDetail `json:"detail"`
}
type SNSPayloadLifecycleDetail struct {
	LifecycleActionToken string `json:"LifecycleActionToken"`
	AutoScalingGroupName string `json:"AutoScalingGroupName"`
	LifecycleHookName    string `json:"LifecycleHookName"`
	EC2InstanceId        string `json:"EC2InstanceId"`
	LifecycleTransition  string `json:"LifecycleTransition"`
}

func (a *API) Launch() error {
	if util.GetEnv("SAML_ENABLED", "") == "yes" {
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
	a.samlHelper, err = newSAML(util.GetEnv("SAML_METADATA_URL", ""), []byte(util.GetEnv("SAML_CERTIFICATE", "")), []byte(util.GetEnv("SAML_PRIVATE_KEY", "")))
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

	// cookie sessions
	r.Use(session.SessionHandler("ecs-deploy", util.GetEnv("JWT_SECRET", "unsecure secret key 8a045eb")))

	// prefix
	prefix := util.GetEnv("URL_PREFIX", "")
	apiPrefix := prefix + util.GetEnv("URL_PREFIX_API", "/api/v1")

	// webapp
	r.Use(ngserve.ServeWithDefault(prefix+"/webapp", ngserve.LocalFile("./webapp/dist", false), "./webapp/dist/index.html"))

	auth := r.Group(apiPrefix)
	auth.Use(a.authMiddleware.MiddlewareFunc())
	{
		// frontend redirect
		r.GET(prefix, a.redirectFrontendHandler)
		r.GET(prefix+"/", a.redirectFrontendHandler)

		// health check
		r.GET(prefix+"/health", a.healthHandler)

		// saml init
		if util.GetEnv("SAML_ENABLED", "") == "yes" {
			r.POST(prefix+"/saml/acs", a.samlHelper.samlInitHandler)
			r.GET(prefix+"/saml/acs", a.samlHelper.samlInitHandler)
		}
		r.GET(prefix+"/saml/enabled", a.samlHelper.samlEnabledHandler)

		// swagger
		r.GET(prefix+"/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

		// webhook
		r.POST(prefix+"/webhook", a.webhookHandler)

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
		auth.POST("/deploy", a.deployServicesHandler)

		// Redeploy existing version
		auth.POST("/deploy/:service/:time", a.redeployServiceHandler)

		// Export
		auth.GET("/export/terraform", a.exportTerraformHandler)
		auth.GET("/export/terraform/:service/targetgrouparn", a.exportTerraformTargetGroupArnHandler)
		auth.GET("/export/terraform/:service/listenerrulearn", a.exportTerraformListenerRuleArnsHandler)
		auth.GET("/export/terraform/:service/listenerrulearn/:rule", a.exportTerraformListenerRuleArnHandler)

		// deploy list
		auth.GET("/deploy/list", a.listDeploysHandler)
		auth.GET("/deploy/list/:service", a.listDeploysForServiceHandler)
		auth.GET("/deploy/status/:service/:time", a.getDeploymentStatusHandler)
		auth.GET("/deploy/get/:service/:time", a.getDeploymentHandler)
		// service list
		auth.GET("/service/list", a.listServicesHandler)
		// service list
		auth.GET("/service/describe", a.describeServicesHandler)
		// get service information
		auth.GET("/service/describe/:service", a.describeServiceHandler)
		// get version information
		auth.GET("/service/describe/:service/versions", a.describeServiceVersionsHandler)
		// scale service
		auth.POST("/service/scale/:service/:count", a.scaleServiceHandler)
		// run task
		auth.POST("/service/runtask/:service", a.runTaskHandler)
		// get taskdefinition
		auth.GET("/service/describe/:service/taskdefinition", a.describeServiceTaskdefinitionHandler)
		// get all tasks
		auth.GET("/service/describe/:service/tasks", a.describeTasksHandler)

		// parameter store
		auth.GET("/service/parameter/:service/list", a.listServiceParametersHandler)
		auth.POST("/service/parameter/:service/put", a.putServiceParameterHandler)
		auth.POST("/service/parameter/:service/delete/:parameter", a.deleteServiceParameterHandler)

		// cloudwatch logs
		auth.GET("/service/log/:service/get/:taskarn/:container/:start/:end", a.getServiceLogsHandler)
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
		Key:              []byte(util.GetEnv("JWT_SECRET", "unsecure secret key 8a045eb")),
		SigningAlgorithm: "HS256",
		Timeout:          time.Hour,
		MaxRefresh:       time.Hour,
		Authenticator: func(userId string, password string, c *gin.Context) (string, bool) {
			if (userId == "deploy" && password == util.GetEnv("DEPLOY_PASSWORD", "deploy")) || (userId == "developer" && password == util.GetEnv("DEVELOPER_PASSWORD", "developer")) {
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
	controller.SetDeployDefaults(&json)
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

// @summary Deploy services to ECS
// @description Deploy services to ECS
// @id ecs-deploy-service
// @accept  json
// @produce  json
// @router /api/v1/deploy [post]
func (a *API) deployServicesHandler(c *gin.Context) {
	var json DeployServices
	var errors map[string]string
	var results []*DeployResult
	var err error
	var res *DeployResult
	var failures int
	errors = make(map[string]string)
	controller := Controller{}
	if err = c.ShouldBindJSON(&json); err == nil {
		for i, v := range json.Services {
			controller.SetDeployDefaults(&json.Services[i])
			if err = a.deployServiceValidator(v.ServiceName, json.Services[i]); err == nil {
				res, err = controller.deploy(v.ServiceName, json.Services[i])
				if err == nil {
					results = append(results, res)
				}
			}
			if err != nil {
				failures += 1
				errors[v.ServiceName] = err.Error()
			}
		}
		c.JSON(200, gin.H{
			"messages": results,
			"failures": failures,
			"errors":   errors,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

// @summary Redeploy existing service to ECS
// @description Redeploy existing service to ECS
// @id ecs-redeploy-service
// @accept  json
// @produce  json
// @param   service         path    string     true        "service name"
// @param   time            path    time       true        "timestamp"
// @router /api/v1/deploy/{service} [post]
func (a *API) redeployServiceHandler(c *gin.Context) {
	controller := Controller{}
	res, err := controller.redeploy(c.Param("service"), c.Param("time"))
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

func (a *API) deployServiceValidator(serviceName string, d Deploy) error {
	if len(serviceName) < 3 {
		return errors.New("service name needs to be at least 3 characters")
	}
	if strings.ToLower(d.ServiceProtocol) != "none" && d.ServicePort == 0 {
		return errors.New("ServicePort needs to be set if ServiceProtocol is not set to none.")
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
func (a *API) describeServiceVersionsHandler(c *gin.Context) {
	controller := Controller{}
	versions, err := controller.describeServiceVersions(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"versions": versions,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) getDeploymentStatusHandler(c *gin.Context) {
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
func (a *API) getDeploymentHandler(c *gin.Context) {
	controller := Controller{}
	deployment, err := controller.getDeployment(c.Param("service"), c.Param("time"))
	if err == nil {
		c.JSON(200, gin.H{
			"deployment": deployment,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}

func (a *API) redirectFrontendHandler(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, util.GetEnv("URL_PREFIX", "")+"/webapp/")
}

func (a *API) listServiceParametersHandler(c *gin.Context) {
	var creds string
	claims := jwt.ExtractClaims(c)
	controller := Controller{}
	session := session.RetrieveSession(c)
	if c, ok := session.Get("paramstore_creds").(string); ok {
		creds = c
	}
	parameters, creds, err := controller.getServiceParameters(c.Param("service"), claims["id"].(string), creds)
	session.Set("paramstore_creds", creds)
	session.Save()
	if err == nil {
		c.JSON(200, gin.H{
			"parameters": parameters,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) putServiceParameterHandler(c *gin.Context) {
	var json DeployServiceParameter
	var creds string
	claims := jwt.ExtractClaims(c)
	controller := Controller{}
	session := session.RetrieveSession(c)
	if c, ok := session.Get("paramstore_creds").(string); ok {
		creds = c
	}
	if err := c.ShouldBindJSON(&json); err == nil {
		res, creds, err := controller.putServiceParameter(c.Param("service"), claims["id"].(string), creds, json)
		session.Set("paramstore_creds", creds)
		session.Save()
		if err == nil {
			c.JSON(200, gin.H{
				"parameters": res,
			})
		} else {
			c.JSON(200, gin.H{
				"error": err.Error(),
			})
		}
	} else {
		c.JSON(200, gin.H{
			"error": "Invalid input",
		})
	}
}
func (a *API) deleteServiceParameterHandler(c *gin.Context) {
	var creds string
	claims := jwt.ExtractClaims(c)
	controller := Controller{}
	session := session.RetrieveSession(c)
	if c, ok := session.Get("paramstore_creds").(string); ok {
		creds = c
	}
	creds, err := controller.deleteServiceParameter(c.Param("service"), claims["id"].(string), creds, c.Param("parameter"))
	session.Set("paramstore_creds", creds)
	session.Save()
	if err == nil {
		c.JSON(200, gin.H{
			"message": "OK",
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) scaleServiceHandler(c *gin.Context) {
	controller := Controller{}
	desiredCount, err := strconv.ParseInt(c.Param("count"), 10, 64)
	if err != nil {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
		return
	}
	err = controller.scaleService(c.Param("service"), desiredCount)
	if err == nil {
		c.JSON(200, gin.H{
			"message": "OK",
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}

func (a *API) webhookHandler(c *gin.Context) {
	controller := Controller{}
	var err error

	snsMessageType := c.GetHeader("x-amz-sns-message-type")
	apiLogger.Debugf("Checking message type: %v", snsMessageType)
	var snsPayload sns.Payload
	if err = c.ShouldBindJSON(&snsPayload); err == nil {
		err = snsPayload.VerifyPayload()
		if err == nil {
			apiLogger.Debugf("Verified Payload.")
			if snsMessageType == "SubscriptionConfirmation" {
				apiLogger.Debugf("Subscribing...")
				_, err = snsPayload.Subscribe()
			} else if snsMessageType == "Notification" {
				apiLogger.Debugf("Incoming Notification")
				var genericMessage SNSPayloadGeneric
				if err = json.Unmarshal([]byte(snsPayload.Message), &genericMessage); err == nil {
					apiLogger.Debugf("Message detail type: %v", genericMessage.DetailType)
					if genericMessage.DetailType == "ECS Container Instance State Change" {
						var ecsMessage SNSPayloadEcs
						if err = json.Unmarshal([]byte(snsPayload.Message), &ecsMessage); err == nil {
							apiLogger.Debugf("ECS Message: %v", snsPayload.Message)
							err = controller.processEcsMessage(ecsMessage)
						}
					} else if genericMessage.DetailType == "EC2 Instance-terminate Lifecycle Action" {
						var lifecycleMessage SNSPayloadLifecycle
						if err = json.Unmarshal([]byte(snsPayload.Message), &lifecycleMessage); err == nil {
							apiLogger.Debugf("Lifecycle Message: %v", snsPayload.Message)
							err = controller.processLifecycleMessage(lifecycleMessage)
						}
					}
				}
			} else {
				err = errors.New("MessageType not recognized")
			}
		}
	}
	if err == nil {
		c.JSON(200, gin.H{
			"message": "OK",
		})
	} else {
		apiLogger.Errorf("Error: %v", err.Error())
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) runTaskHandler(c *gin.Context) {
	var json RunTask
	controller := Controller{}
	if err := c.ShouldBindJSON(&json); err == nil {
		claims := jwt.ExtractClaims(c)
		json.StartedBy = strings.Replace(claims["id"].(string), "@", "-", -1)
		taskArn, err := controller.runTask(c.Param("service"), json)
		if err == nil {
			c.JSON(200, gin.H{
				"taskArn": taskArn,
			})
		} else {
			c.JSON(200, gin.H{
				"error": err.Error(),
			})
		}
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) describeServiceTaskdefinitionHandler(c *gin.Context) {
	controller := Controller{}
	taskDefinition, err := controller.describeTaskDefinition(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"taskDefinition": taskDefinition,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) describeTasksHandler(c *gin.Context) {
	controller := Controller{}
	taskArns, err := controller.listTasks(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"tasks": taskArns,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
func (a *API) getServiceLogsHandler(c *gin.Context) {
	controller := Controller{}
	layout := "2006-01-02T15:04:05.9Z"
	start, err := time.Parse(layout, c.Param("start"))
	if err != nil {
		c.JSON(200, gin.H{
			"error": "Can't parse start date: " + err.Error(),
		})
		return
	}
	end, err := time.Parse(layout, c.Param("end"))
	if err != nil {
		c.JSON(200, gin.H{
			"error": "Can't parse end date",
		})
		return
	}
	logs, err := controller.getServiceLogs(c.Param("service"), c.Param("taskarn"), c.Param("container"), start, end)
	if err == nil {
		c.JSON(200, gin.H{
			"logs": logs,
		})
	} else {
		c.JSON(200, gin.H{
			"error": err.Error(),
		})
	}
}
