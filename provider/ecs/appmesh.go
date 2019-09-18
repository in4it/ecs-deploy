package ecs

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/appmesh"
	"github.com/in4it/ecs-deploy/service"
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

func (a *AppMesh) listVirtualNodes(meshName string) (map[string]string, error) {
	svc := appmesh.New(session.New())
	pageNum := 0
	result := make(map[string]string)
	input := &appmesh.ListVirtualNodesInput{
		MeshName: aws.String(meshName),
	}
	err := svc.ListVirtualNodesPages(input,
		func(page *appmesh.ListVirtualNodesOutput, lastPage bool) bool {
			pageNum++
			for _, virtualNode := range page.VirtualNodes {
				result[aws.StringValue(virtualNode.VirtualNodeName)] = aws.StringValue(virtualNode.Arn)
			}
			return pageNum <= 100
		})
	if err != nil {
		appmeshLogger.Errorf(err.Error())
		return result, err
	}
	return result, nil
}

func (a *AppMesh) listVirtualServices(meshName string) (map[string]string, error) {
	svc := appmesh.New(session.New())
	pageNum := 0
	result := make(map[string]string)
	input := &appmesh.ListVirtualServicesInput{
		MeshName: aws.String(meshName),
	}
	err := svc.ListVirtualServicesPages(input,
		func(page *appmesh.ListVirtualServicesOutput, lastPage bool) bool {
			pageNum++
			for _, virtualNode := range page.VirtualServices {
				result[aws.StringValue(virtualNode.VirtualServiceName)] = aws.StringValue(virtualNode.Arn)
			}
			return pageNum <= 100
		})
	if err != nil {
		appmeshLogger.Errorf(err.Error())
		return result, err
	}
	return result, nil
}

func (a *AppMesh) listVirtualRouters(meshName string) (map[string]string, error) {
	svc := appmesh.New(session.New())
	pageNum := 0
	result := make(map[string]string)
	input := &appmesh.ListVirtualRoutersInput{
		MeshName: aws.String(meshName),
	}
	err := svc.ListVirtualRoutersPages(input,
		func(page *appmesh.ListVirtualRoutersOutput, lastPage bool) bool {
			pageNum++
			for _, virtualRouter := range page.VirtualRouters {
				result[aws.StringValue(virtualRouter.VirtualRouterName)] = aws.StringValue(virtualRouter.Arn)
			}
			return pageNum <= 100
		})
	if err != nil {
		appmeshLogger.Errorf(err.Error())
		return result, err
	}
	return result, nil
}

func (a *AppMesh) createVirtualNode(virtualNodeName, virtualNodeDNS, meshName string, servicePort int64, healthcheck AppMeshHealthCheck, backends []string) error {
	var appmeshBackends []*appmesh.Backend

	for _, backend := range backends {
		appmeshBackends = append(appmeshBackends, &appmesh.Backend{
			VirtualService: &appmesh.VirtualServiceBackend{
				VirtualServiceName: aws.String(backend),
			},
		})
	}

	svc := appmesh.New(session.New())
	input := &appmesh.CreateVirtualNodeInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualNodeSpec{
			Backends: appmeshBackends,
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
					Hostname: aws.String(virtualNodeDNS),
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

func (a *AppMesh) updateVirtualNode(virtualNodeName, virtualNodeDNS, meshName string, servicePort int64, healthcheck AppMeshHealthCheck, backends []string) error {
	var appmeshBackends []*appmesh.Backend

	for _, backend := range backends {
		appmeshBackends = append(appmeshBackends, &appmesh.Backend{
			VirtualService: &appmesh.VirtualServiceBackend{
				VirtualServiceName: aws.String(backend),
			},
		})
	}

	svc := appmesh.New(session.New())
	input := &appmesh.UpdateVirtualNodeInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualNodeSpec{
			Backends: appmeshBackends,
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
					Hostname: aws.String(virtualNodeDNS),
				},
			},
		},
		VirtualNodeName: aws.String(virtualNodeName),
	}

	_, err := svc.UpdateVirtualNode(input)
	if err != nil {
		return err
	}

	return nil
}

func (a *AppMesh) createVirtualServiceWithVirtualNode(virtualServiceName, virtualNodeName, meshName string) error {
	svc := appmesh.New(session.New())
	input := &appmesh.CreateVirtualServiceInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualServiceSpec{
			Provider: &appmesh.VirtualServiceProvider{
				VirtualNode: &appmesh.VirtualNodeServiceProvider{
					VirtualNodeName: aws.String(virtualNodeName),
				},
			},
		},
		VirtualServiceName: aws.String(virtualServiceName),
	}

	_, err := svc.CreateVirtualService(input)
	if err != nil {
		return err
	}

	return nil
}

func (a *AppMesh) createVirtualServiceWithVirtualRouter(virtualServiceName, virtualRouterName, meshName string) error {
	svc := appmesh.New(session.New())
	input := &appmesh.CreateVirtualServiceInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualServiceSpec{
			Provider: &appmesh.VirtualServiceProvider{
				VirtualRouter: &appmesh.VirtualRouterServiceProvider{
					VirtualRouterName: aws.String(virtualRouterName),
				},
			},
		},
		VirtualServiceName: aws.String(virtualServiceName),
	}

	_, err := svc.CreateVirtualService(input)
	if err != nil {
		return err
	}

	return nil
}

func (a *AppMesh) updateVirtualServiceWithVirtualRouter(virtualServiceName, virtualRouterName, meshName string) error {
	svc := appmesh.New(session.New())
	input := &appmesh.UpdateVirtualServiceInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualServiceSpec{
			Provider: &appmesh.VirtualServiceProvider{
				VirtualRouter: &appmesh.VirtualRouterServiceProvider{
					VirtualRouterName: aws.String(virtualRouterName),
				},
			},
		},
		VirtualServiceName: aws.String(virtualServiceName),
	}

	_, err := svc.UpdateVirtualService(input)
	if err != nil {
		return err
	}

	return nil
}

func (a *AppMesh) createVirtualRouter(virtualRouterName string, meshName string, servicePort int64) error {
	svc := appmesh.New(session.New())
	input := &appmesh.CreateVirtualRouterInput{
		MeshName: aws.String(meshName),
		Spec: &appmesh.VirtualRouterSpec{
			Listeners: []*appmesh.VirtualRouterListener{
				{
					PortMapping: &appmesh.PortMapping{
						Port:     aws.Int64(servicePort),
						Protocol: aws.String("http"),
					},
				},
			},
		},
		VirtualRouterName: aws.String(virtualRouterName),
	}

	_, err := svc.CreateVirtualRouter(input)
	if err != nil {
		return err
	}

	return nil
}

func (a *AppMesh) createRoute(routeName, virtualRouterName, virtualNodeName, hostname string, mesh service.DeployAppMesh) error {
	perRetryTimeout, err := time.ParseDuration(mesh.RetryPolicy.PerRetryTimeout)
	if err != nil {
		return err
	}
	svc := appmesh.New(session.New())
	input := &appmesh.CreateRouteInput{
		MeshName: aws.String(mesh.Name),
		Spec: &appmesh.RouteSpec{
			HttpRoute: &appmesh.HttpRoute{
				Match: &appmesh.HttpRouteMatch{
					Prefix: aws.String("/"),
					/*Headers: []*appmesh.HttpRouteHeader{
						{
							Name: aws.String("Host"),
							Match: &appmesh.HeaderMatchMethod{
								Exact: aws.String(hostname),
							},
						},
					},*/
				},
				Action: &appmesh.HttpRouteAction{
					WeightedTargets: []*appmesh.WeightedTarget{
						{
							VirtualNode: aws.String(virtualNodeName),
							Weight:      aws.Int64(100),
						},
					},
				},
				RetryPolicy: &appmesh.HttpRetryPolicy{
					HttpRetryEvents: aws.StringSlice(mesh.RetryPolicy.HTTPRetryEvents),
					MaxRetries:      aws.Int64(mesh.RetryPolicy.MaxRetries),
					PerRetryTimeout: &appmesh.Duration{
						Unit:  aws.String("ms"),
						Value: aws.Int64(perRetryTimeout.Milliseconds()),
					},
					TcpRetryEvents: aws.StringSlice(mesh.RetryPolicy.TcpRetryEvents),
				},
			},
			Priority: aws.Int64(100),
		},
		RouteName:         aws.String(routeName),
		VirtualRouterName: aws.String(virtualRouterName),
	}

	_, err = svc.CreateRoute(input)
	if err != nil {
		return err
	}

	return nil
}
