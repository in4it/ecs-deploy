package service

import (
	"time"
)

// deploy binding from JSON
type DeployServices struct {
	Services []Deploy `json:"services" yaml:"services" binding:"required"`
}
type Deploy struct {
	Cluster               string                      `json:"cluster" yaml:"cluster" binding:"required"`
	LoadBalancer          string                      `json:"loadBalancer" yaml:"loadBalancer"`
	ServiceName           string                      `json:"serviceName" yaml:"serviceName"`
	ServicePort           int64                       `json:"servicePort" yaml:"servicePort"`
	ServiceProtocol       string                      `json:"serviceProtocol" yaml:"serviceProtocol" binding:"required"`
	DesiredCount          int64                       `json:"desiredCount" yaml:"desiredCount" binding:"required"`
	MinimumHealthyPercent int64                       `json:"minimumHealthyPercent" yaml:"minimumHealthyPercent"`
	MaximumPercent        int64                       `json:"maximumPercent" yaml:"maximumPercent"`
	Containers            []*DeployContainer          `json:"containers" yaml:"containers" binding:"required,dive"`
	HealthCheck           DeployHealthCheck           `json:"healthCheck" yaml:"healthCheck"`
	RuleConditions        []*DeployRuleConditions     `json:"ruleConditions" yaml:"ruleConditions"`
	NetworkMode           string                      `json:"networkMode" yaml:"networkMode"`
	NetworkConfiguration  DeployNetworkConfiguration  `json:"networkConfiguration" yaml:"networkConfiguration"`
	PlacementConstraints  []DeployPlacementConstraint `json:"placementConstraints" yaml:"placementConstraints"`
	LaunchType            string                      `json:"launchType" yaml:"launchType"`
	DeregistrationDelay   int64                       `json:"deregistrationDelay" yaml:"deregistrationDelay"`
	Stickiness            DeployStickiness            `json:"stickiness" yaml:"stickiness"`
	Volumes               []DeployVolume              `json:"volumes" yaml:"volumes"`
	EnvNamespace          string                      `json:"envNamespace" yaml:"envNamespace"`
	ServiceRegistry       string                      `json:"serviceRegistry" yaml:"serviceRegistry"`
	SchedulingStrategy    string                      `json:"schedulingStrategy" yaml:"schedulingStrategy"`
}
type DeployContainer struct {
	ContainerName       string                        `json:"containerName" yaml:"containerName" binding:"required"`
	ContainerTag        string                        `json:"containerTag" yaml:"containerTag" binding:"required"`
	ContainerPort       int64                         `json:"containerPort" yaml:"containerPort"`
	ContainerCommand    []*string                     `json:"containerCommand" yaml:"containerCommand"`
	ContainerImage      string                        `json:"containerImage" yaml:"containerImage"`
	ContainerURI        string                        `json:"containerURI" yaml:"containerURI"`
	ContainerEntryPoint []*string                     `json:"containerEntryPoint" yaml:"containerEntryPoint"`
	Essential           bool                          `json:"essential" yaml:"essential"`
	Memory              int64                         `json:"memory" yaml:"memory"`
	MemoryReservation   int64                         `json:"memoryReservation" yaml:"memoryReservation"`
	CPU                 int64                         `json:"cpu" yaml:"cpu"`
	CPUReservation      int64                         `json:"cpuReservation" yaml:"cpuReservation"`
	DockerLabels        map[string]string             `json:"dockerLabels" yaml:"dockerLabels"`
	HealthCheck         DeployContainerHealthCheck    `json:"healthCheck" yaml:"healthCheck"`
	Environment         []*DeployContainerEnvironment `json:"environment" yaml:"environment"`
	MountPoints         []*DeployContainerMountPoint  `json:"mountPoints" yaml:"mountPoints"`
	Ulimits             []*DeployContainerUlimit      `json:"ulimits" yaml:"ulimits"`
	Links               []*string                     `json:"links" yaml:"links"`
	LogConfiguration    DeployLogConfiguration        `json:"logConfiguration" yaml:"logConfiguration"`
	PortMappings        []DeployContainerPortMapping  `json:"portMappings" yaml:"portMappings"`
}
type DeployContainerPortMapping struct {
	Protocol      string `json:"protocol" yaml:"protocol"`
	HostPort      int64  `json:"hostPort" yaml:"hostPort"`
	ContainerPort int64  `json:"containerPort" yaml:"containerPort"`
}
type DeployLogConfiguration struct {
	LogDriver string                        `json:"logDriver" yaml:"logDriver"`
	Options   DeployLogConfigurationOptions `json:"options" yaml:"options"`
}
type DeployLogConfigurationOptions struct {
	MaxSize string `json:"max-size" yaml:"max-size"`
	MaxFile string `json:"max-file" yaml:"max-file"`
}
type DeployContainerUlimit struct {
	Name      string `json:"name" yaml:"name"`
	SoftLimit int64  `json:"softLimit" yaml:"softLimit"`
	HardLimit int64  `json:"hardLimit" yaml:"hardLimit"`
}
type DeployContainerEnvironment struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}
type DeployContainerMountPoint struct {
	ContainerPath string `json:"containerPath" yaml:"containerPath"`
	SourceVolume  string `json:"sourceVolume" yaml:"sourceVolume"`
	ReadOnly      bool   `json:"readonly" yaml:"readonly"`
}
type DeployContainerHealthCheck struct {
	Command     []*string `json:"command" yaml:"command"`
	Interval    int64     `json:"interval" yaml:"interval"`
	Timeout     int64     `json:"timeout" yaml:"timeout"`
	Retries     int64     `json:"retries" yaml:"retries"`
	StartPeriod int64     `json:"startPeriod" yaml:"startPeriod"`
}
type DeployNetworkConfiguration struct {
	AssignPublicIp string   `json:"assignPublicIp" yaml:"assignPublicIp"`
	SecurityGroups []string `json:"securityGroups" yaml:"securityGroups"`
	Subnets        []string `json:"subnets" yaml:"subnets"`
}
type DeployPlacementConstraint struct {
	Expression string `json:"expression" yaml:"expression"`
	Type       string `json:"type" yaml:"type"`
}
type DeployHealthCheck struct {
	HealthyThreshold   int64  `json:"healthyThreshold" yaml:"healthyThreshold"`
	UnhealthyThreshold int64  `json:"unhealthyThreshold" yaml:"unhealthyThreshold"`
	Path               string `json:"path" yaml:"path"`
	Port               string `json:"port" yaml:"port"`
	Protocol           string `json:"protocol" yaml:"protocol"`
	Interval           int64  `json:"interval" yaml:"interval"`
	Matcher            string `json:"matcher" yaml:"matcher"`
	Timeout            int64  `json:"timeout" yaml:"timeout"`
	GracePeriodSeconds int64  `json:"gracePeriodSeconds" yaml:"gracePeriodSeconds"`
}
type DeployRuleConditions struct {
	Listeners   []string                        `json:"listeners" yaml:"listeners"`
	PathPattern string                          `json:"pathPattern" yaml:"pathPattern"`
	Hostname    string                          `json:"hostname" yaml:"hostname"`
	CognitoAuth DeployRuleConditionsCognitoAuth `json:"cognitoAuth" yaml:"cognitoAuth"`
}
type DeployRuleConditionsCognitoAuth struct {
	UserPoolName string `json:"userPoolName" yaml:"userPoolName"`
	ClientName   string `json:"clientName" yaml:"clientName"`
}
type DeployStickiness struct {
	Enabled  bool  `json:"enabled" yaml:"enabled"`
	Duration int64 `json:"duration" yaml:"duration"`
}
type DeployVolume struct {
	Host                      DeployVolumeHost                      `json:"host" yaml:"host"`
	DockerVolumeConfiguration DeployVolumeDockerVolumeConfiguration `json:"dockerVolumeConfiguration" yaml:"dockerVolumeConfiguration"`
	Name                      string                                `json:"name" yaml:"name"`
}
type DeployVolumeHost struct {
	SourcePath string `json:"sourcePath" yaml:"sourcePath"`
}
type DeployVolumeDockerVolumeConfiguration struct {
	Scope         string            `json:"scope" yaml:"scope"`
	Autoprovision bool              `json:"autoprovision" yaml:"autoprovision"`
	Driver        string            `json:"driver" yaml:"driver"`
	DriverOpts    map[string]string `json:"driverOpts" yaml:"driverOpts"`
	Labels        map[string]string `json:"labels" yaml:"labels"`
}

type DeployResult struct {
	ServiceName       string    `json:"serviceName" yaml:"serviceName"`
	ClusterName       string    `json:"clusterName" yaml:"clusterName"`
	TaskDefinitionArn string    `json:"taskDefinitionArn" yaml:"taskDefinitionArn"`
	Status            string    `json:"status" yaml:"status"`
	DeployError       string    `json:"deployError" yaml:"deployError"`
	DeploymentTime    time.Time `json:"deploymentTime" yaml:"deploymentTime"`
}
type DeployServiceParameter struct {
	Name      string `json:"name" yaml:"name" binding:"required"`
	Value     string `json:"value" yaml:"value" binding:"required"`
	Encrypted bool   `json:"encrypted" yaml:"encrypted"`
}

type RunningService struct {
	ServiceName  string                     `json:"serviceName" yaml:"serviceName"`
	ClusterName  string                     `json:"clusterName" yaml:"clusterName"`
	RunningCount int64                      `json:"runningCount" yaml:"runningCount"`
	PendingCount int64                      `json:"pendingCount" yaml:"pendingCount"`
	DesiredCount int64                      `json:"desiredCount" yaml:"desiredCount"`
	Status       string                     `json:"status" yaml:"status"`
	Events       []RunningServiceEvent      `json:"events" yaml:"events"`
	Deployments  []RunningServiceDeployment `json:"deployments" yaml:"deployments"`
	Tasks        []RunningTask              `json:"tasks" yaml:"tasks"`
}
type RunningServiceDeployment struct {
	Status         string    `json:"status" yaml:"status"`
	RunningCount   int64     `json:"runningCount" yaml:"runningCount"`
	PendingCount   int64     `json:"pendingCount" yaml:"pendingCount"`
	DesiredCount   int64     `json:"desiredCount" yaml:"desiredCount"`
	CreatedAt      time.Time `json:"createdAt" yaml:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt" yaml:"updatedAt"`
	TaskDefinition string    `json:"taskDefinition" yaml:"taskDefinition"`
}
type RunningServiceEvent struct {
	CreatedAt time.Time `json:"createdAt" yaml:"createdAt"`
	Id        string    `json:"id" yaml:"id"`
	Message   string    `json:"message" yaml:"message"`
}
type ServiceVersion struct {
	ImageName  string    `json:"imageName" yaml:"imageName"`
	Tag        string    `json:"tag" yaml:"tag"`
	ImageId    string    `json:"imageId" yaml:"imageId"`
	LastDeploy time.Time `json:"lastDeploy" yaml:"lastDeploy"`
}
type RunningTask struct {
	ContainerInstanceArn string                 `json:"containerInstanceArn" yaml:"containerInstanceArn"`
	Containers           []RunningTaskContainer `json:"containers" yaml:"containers"`
	Cpu                  string                 `json:"cpu" yaml:"cpu"`
	CreatedAt            time.Time              `json:"createdAt" yaml:"createdAt"`
	DesiredStatus        string                 `json:"desiredStatus" yaml:"desiredStatus"`
	ExecutionStoppedAt   time.Time              `json:"executionStoppedAt" yaml:"executionStoppedAt"`
	Group                string                 `json:"group" yaml:"group"`
	LastStatus           string                 `json:"lastStatus" yaml:"lastStatus"`
	LaunchType           string                 `json:"launchType" yaml:"launchType"`
	Memory               string                 `json:"memory" yaml:"memory"`
	PullStartedAt        time.Time              `json:"pullStartedAt" yaml:"pullStartedAt"`
	PullStoppedAt        time.Time              `json:"pullStoppedAt" yaml:"pullStoppedAt"`
	StartedAt            time.Time              `json:"startedAt" yaml:"startedAt"`
	StartedBy            string                 `json:"startedBy" yaml:"startedBy"`
	StoppedAt            time.Time              `json:"stoppedAt" yaml:"stoppedAt"`
	StoppedReason        string                 `json:"stoppedReason" yaml:"stoppedReason"`
	StoppingAt           time.Time              `json:"stoppingAt" yaml:"stoppingAt"`
	TaskArn              string                 `json:"taskArn" yaml:"taskArn"`
	TaskDefinitionArn    string                 `json:"taskDefinitionArn" yaml:"taskDefinitionArn"`
	Version              int64                  `json:"version" yaml:"version"`
}
type RunningTaskContainer struct {
	ContainerArn string `json:"containerArn" yaml:"containerArn"`
	ExitCode     int64  `json:"exitCode" yaml:"exitCode"`
	LastStatus   string `json:"lastStatus" yaml:"lastStatus"`
	Name         string `json:"name" yaml:"name"`
	Reason       string `json:"reason" yaml:"reason"`
}

// "Run ad-hoc task" type
type RunTask struct {
	StartedBy          string                     `json:"startedBy" yaml:"startedBy"`
	ContainerOverrides []RunTaskContainerOverride `json:"containerOverrides" yaml:"containerOverrides"`
}
type RunTaskContainerOverride struct {
	Name        string                        `json:"name" yaml:"name"`
	Command     []string                      `json:"command" yaml:"command"`
	Environment []*DeployContainerEnvironment `json:"environment" yaml:"environment"`
}

// create Autoscaling Policy
type Autoscaling struct {
	MinimumCount int64               `json:"minimumCount" yaml:"minimumCount"`
	DesiredCount int64               `json:"desiredCount" yaml:"desiredCount"`
	MaximumCount int64               `json:"maximumCount" yaml:"maximumCount"`
	Policies     []AutoscalingPolicy `json:"policies" yaml:"policies"`
}
type AutoscalingPolicy struct {
	PolicyName           string  `json:"policyName" yaml:"policyName"`
	ComparisonOperator   string  `json:"comparisonOperator" yaml:"comparisonOperator"`
	Metric               string  `json:"metric" yaml:"metric"`
	NewAutoscalingPolicy bool    `json:"newAutoscalingPolicy" yaml:"newAutoscalingPolicy"`
	Threshold            float64 `json:"threshold" yaml:"threshold"`
	ScalingAdjustment    int64   `json:"scalingAdjustment" yaml:"scalingAdjustment"`
	ThresholdStatistic   string  `json:"thresholdStatistic" yaml:"thresholdStatistic"`
	DatapointsToAlarm    int64   `json:"datapointsToAlarm" yaml:"datapointsToAlarm"`
	EvaluationPeriods    int64   `json:"evaluationPeriods" yaml:"evaluationPeriods"`
	Period               int64   `json:"period" yaml:"period"`
}

type LoadBalancer struct {
	Name          string
	IPAddressType string
	Scheme        string
	Type          string
}
