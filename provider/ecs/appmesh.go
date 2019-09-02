package ecs

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/appmesh"
	"github.com/juju/loggo"
)

// logging
var appmeshLogger = loggo.GetLogger("appmesh")

// AppMesh struct
type AppMesh struct {
}

//AppMeshHealthCheck is a struct that contains the healthcheck for the appmesh
type AppMeshHealthCheck struct {
	HealthyThreshold   int64
	IntervalMillis     int64
	Path               string
	Port               int64
	Protocol           string
	TimeoutMillis      int64
	UnhealthyThreshold int64
}

func (a *AppMesh) createVirtualNodeName(virtualNodeName, meshName string, servicePort int64, healthcheck AppMeshHealthCheck) error {
	svc := appmesh.New(session.New())
	input := &appmesh.CreateVirtualNodeInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualNodeSpec{
			Listeners: []*appmesh.Listener{
				{
					HealthCheck: &appmesh.HealthCheckPolicy{
						HealthyThreshold:   aws.Int64(healthcheck.HealthyThreshold),
						IntervalMillis:     aws.Int64(healthcheck.IntervalMillis),
						Path:               aws.String(healthcheck.Path),
						Port:               aws.Int64(healthcheck.Port),
						Protocol:           aws.String(healthcheck.Protocol),
						TimeoutMillis:      aws.Int64(healthcheck.TimeoutMillis),
						UnhealthyThreshold: aws.Int64(healthcheck.UnhealthyThreshold),
					},
					PortMapping: &appmesh.PortMapping{
						Port:     aws.Int64(servicePort),
						Protocol: aws.String("http"),
					},
				},
			},
			ServiceDiscovery: &appmesh.ServiceDiscovery{
				Dns: &appmesh.DnsServiceDiscovery{
					Hostname: aws.String(virtualNodeName),
				},
			},
		},
		VirtualNodeName: aws.String(virtualNodeName),
	}

	_, err := svc.CreateVirtualNode(input)
	if err != nil {
		return err
	}

	return nil
}
