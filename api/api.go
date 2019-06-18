package api

import (
	//"github.com/RobotsAndPencils/go-saml"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/in4it/ecs-deploy/docs"
	"github.com/in4it/ecs-deploy/ngserve"
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
	"github.com/in4it/ecs-deploy/session"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
	sns "github.com/robbiet480/go.sns"
	ginSwagger "github.com/swaggo/gin-swagger"   // gin-swagger middleware
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

type User struct {
	UserID string
}
type login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
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

		// service autoscaling
		auth.POST("/service/autoscaling/:service/put", a.putServiceAutoscalingHandler)
		auth.GET("/service/autoscaling/:service/get", a.getServiceAutoscalingHandler)
		auth.POST("/service/autoscaling/:service/delete/:policyname", a.deleteServiceAutoscalingPolicyHandler)
		auth.POST("/service/autoscaling/:service/delete", a.deleteServiceAutoscalingHandler)
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
	var (
		identityKey = "id"
		err         error
	)
	a.authMiddleware, err = jwt.New(&jwt.GinJWTMiddleware{
		Realm:            "ecs-deploy",
		Key:              []byte(util.GetEnv("JWT_SECRET", "unsecure secret key 8a045eb")),
		SigningAlgorithm: "HS256",
		Timeout:          time.Hour,
		MaxRefresh:       time.Hour,
		IdentityKey:      identityKey,
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(*User); ok {
				return jwt.MapClaims{
					identityKey: v.UserID,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &User{
				UserID: claims["id"].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var loginVals login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}
			userID := loginVals.Username
			password := loginVals.Password

			if (userID == "deploy" && password == util.GetEnv("DEPLOY_PASSWORD", "deploy")) || (userID == "developer" && password == util.GetEnv("DEVELOPER_PASSWORD", "developer")) {
				return &User{UserID: userID}, nil
			}

			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			if v, ok := data.(*User); ok && v.UserID != "" {
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
	})
	if err != nil {
		panic(err)
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
	var json service.Deploy
	controller := Controller{}
	service.SetDeployDefaults(&json)
	if err := c.ShouldBindJSON(&json); err == nil {
		if err = a.deployServiceValidator(c.Param("service"), json); err == nil {
			res, err := controller.Deploy(c.Param("service"), json)
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
	var json service.DeployServices
	var errors map[string]string
	var results []*service.DeployResult
	var err error
	var res *service.DeployResult
	var failures int
	errors = make(map[string]string)
	controller := Controller{}
	if err = c.ShouldBindJSON(&json); err == nil {
		for i, v := range json.Services {
			if err = a.deployServiceValidator(v.ServiceName, json.Services[i]); err == nil {
				res, err = controller.Deploy(v.ServiceName, json.Services[i])
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

func (a *API) deployServiceValidator(serviceName string, d service.Deploy) error {
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
	session, sessionExists := session.RetrieveSession(c)
	if sessionExists {
		if c, ok := session.Get("paramstore_creds").(string); ok {
			creds = c
		}
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
	var json service.DeployServiceParameter
	var creds string
	claims := jwt.ExtractClaims(c)
	controller := Controller{}
	session, sessionExists := session.RetrieveSession(c)
	if sessionExists {
		if c, ok := session.Get("paramstore_creds").(string); ok {
			creds = c
		}
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
	session, sessionExists := session.RetrieveSession(c)
	if sessionExists {
		if c, ok := session.Get("paramstore_creds").(string); ok {
			creds = c
		}
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
	asController := AutoscalingController{}
	var err error

	snsMessageType := c.GetHeader("x-amz-sns-message-type")
	apiLogger.Tracef("Checking message type: %v", snsMessageType)
	var snsPayload sns.Payload
	if err = c.ShouldBindJSON(&snsPayload); err == nil {
		err = snsPayload.VerifyPayload()
		if err == nil {
			apiLogger.Tracef("Verified Payload.")
			if snsMessageType == "SubscriptionConfirmation" {
				apiLogger.Debugf("Subscribing...")
				_, err = snsPayload.Subscribe()
			} else if snsMessageType == "Notification" {
				apiLogger.Debugf("Incoming Notification")
				var genericMessage ecs.SNSPayloadGeneric
				if err = json.Unmarshal([]byte(snsPayload.Message), &genericMessage); err == nil {
					apiLogger.Tracef("Message detail type: %v", genericMessage.DetailType)
					if genericMessage.DetailType == "ECS Container Instance State Change" {
						var ecsMessage ecs.SNSPayloadEcs
						if err = json.Unmarshal([]byte(snsPayload.Message), &ecsMessage); err == nil {
							apiLogger.Tracef("ECS Message: %v", snsPayload.Message)
							err = asController.processEcsMessage(ecsMessage)
						}
					} else if genericMessage.DetailType == "EC2 Instance-terminate Lifecycle Action" {
						var lifecycleMessage ecs.SNSPayloadLifecycle
						if err = json.Unmarshal([]byte(snsPayload.Message), &lifecycleMessage); err == nil {
							apiLogger.Debugf("Lifecycle Message: %v", snsPayload.Message)
							err = asController.processLifecycleMessage(lifecycleMessage)
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
	var json service.RunTask
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
func (a *API) putServiceAutoscalingHandler(c *gin.Context) {
	var json service.Autoscaling
	controller := Controller{}
	if err := c.ShouldBindJSON(&json); err == nil {
		result, err := controller.putServiceAutoscaling(c.Param("service"), json)
		if err == nil {
			c.JSON(200, gin.H{
				"autoscaling": result,
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
func (a *API) getServiceAutoscalingHandler(c *gin.Context) {
	controller := Controller{}
	result, err := controller.getServiceAutoscaling(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"autoscaling": result,
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
func (a *API) deleteServiceAutoscalingPolicyHandler(c *gin.Context) {
	controller := Controller{}
	err := controller.deleteServiceAutoscalingPolicy(c.Param("service"), c.Param("policyname"))
	if err == nil {
		c.JSON(200, gin.H{
			"autoscaling": "deleted",
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}

func (a *API) deleteServiceAutoscalingHandler(c *gin.Context) {
	controller := Controller{}
	err := controller.deleteServiceAutoscaling(c.Param("service"))
	if err == nil {
		c.JSON(200, gin.H{
			"autoscaling": "deleted",
		})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
