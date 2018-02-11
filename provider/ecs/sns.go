package ecs

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
	ClusterArn           string                          `json:"clusterArn"`
	ContainerInstanceArn string                          `json:"containerInstanceArn"`
	Ec2InstanceId        string                          `json:"ec2InstanceId"`
	RegisteredResources  []ContainerInstanceResource     `json:"registeredResources"`
	RemainingResources   []ContainerInstanceResource     `json:"remainingResources"`
	Status               string                          `json:"status"`
	Version              int64                           `json:"version"`
	VersionInfo          EcsVersionInfo                  `json:"versionInfo"`
	UpdatedAt            string                          `json:"updatedAt"`
	RegisteredAt         string                          `json:"registeredAt"`
	Attributes           []SNSPayloadEcsDetailAttributes `json:"attributes"`
}
type SNSPayloadEcsDetailAttributes struct {
	Name  string `json:"name"`
	Value string `json:"value"`
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
