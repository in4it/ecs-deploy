package service

import (
	"time"
)

// deploy binding from JSON
type DeployServices struct {
	Services []Deploy `json:"services" binding:"required"`
}
type Deploy struct {
	Cluster               string                      `json:"cluster" binding:"required"`
	LoadBalancer          string                      `json:"loadBalancer"`
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
	Volumes               []DeployVolume              `json:"volumes"`
}
type DeployContainer struct {
	ContainerName     string                        `json:"containerName" binding:"required"`
	ContainerTag      string                        `json:"containerTag" binding:"required"`
	ContainerPort     int64                         `json:"containerPort"`
	ContainerCommand  []*string                     `json:"containerCommand"`
	ContainerImage    string                        `json:"containerImage`
	ContainerURI      string                        `json:"containerURI"`
	Essential         bool                          `json:"essential"`
	Memory            int64                         `json:"memory"`
	MemoryReservation int64                         `json:"memoryReservation"`
	CPU               int64                         `json:"cpu"`
	CPUReservation    int64                         `json:"cpuReservation"`
	DockerLabels      map[string]string             `json:"dockerLabels"`
	Environment       []*DeployContainerEnvironment `json:"environment"`
	MountPoints       []*DeployContainerMountPoint  `json:"mountPoints"`
	Ulimits           []*DeployContainerUlimit      `json:"ulimits"`
}
type DeployContainerUlimit struct {
	Name      string `json:"name"`
	SoftLimit int64  `json:"softLimit"`
	HardLimit int64  `json:"hardLimit"`
}
type DeployContainerEnvironment struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type DeployContainerMountPoint struct {
	ContainerPath string `json:"containerPath"`
	SourceVolume  string `json:"sourceVolume"`
	ReadOnly      bool   `json:"readonly"`
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
type DeployVolume struct {
	Host DeployVolumeHost `json:"host"`
	Name string           `json:"name"`
}
type DeployVolumeHost struct {
	SourcePath string `json:"sourcePath"`
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
	Name        string                        `json:"name"`
	Command     []string                      `json:"command"`
	Environment []*DeployContainerEnvironment `json:"environment"`
}

// create Autoscaling Policy
type Autoscaling struct {
	MinimumCount int64               `json:"minimumCount"`
	DesiredCount int64               `json:"desiredCount"`
	MaximumCount int64               `json:"maximumCount"`
	Policies     []AutoscalingPolicy `json:"policies"`
}
type AutoscalingPolicy struct {
	PolicyName           string  `json:"policyName"`
	ComparisonOperator   string  `json:"comparisonOperator"`
	Metric               string  `json:"metric"`
	NewAutoscalingPolicy bool    `json:"newAutoscalingPolicy"`
	Threshold            float64 `json:"threshold"`
	ScalingAdjustment    int64   `json:"scalingAdjustment"`
	ThresholdStatistic   string  `json:"thresholdStatistic"`
	DatapointsToAlarm    int64   `json:"datapointsToAlarm"`
	EvaluationPeriods    int64   `json:"evaluationPeriods"`
	Period               int64   `json:"period"`
}

type LoadBalancer struct {
	Name          string
	IPAddressType string
	Scheme        string
	Type          string
}
