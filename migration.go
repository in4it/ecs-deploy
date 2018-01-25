package main

import (
	"github.com/juju/loggo"

	"strconv"
	"strings"
	"time"
)

// logging
var migrationLogger = loggo.GetLogger("migration")

// Migration struct
type Migration struct {
}

func (m *Migration) run(apiVersion string) error {
	var runningMajor, runningMinor int
	v := strings.Split(apiVersion, ".")
	if len(v) > 1 {
		runningMajor, _ = strconv.Atoi(v[0])
		runningMinor, _ = strconv.Atoi(v[1])
	} else {
		runningMajor = 1
		runningMinor = 0
	}
	if runningMajor == 1 && runningMinor < 2 {
		migrationLogger.Infof("Starting migration from %v to %v", apiVersion, getApiVersion())
		var dss DynamoServices
		service := newService()
		ecs := ECS{}
		err := service.getServices(&dss)
		if err != nil {
			return err
		}
		for _, ds := range dss.Services {
			// doing one per half second not to overload db
			service.clusterName = ds.C
			service.serviceName = ds.S
			d, err := service.getLastDeploy()
			if err != nil {
				return err
			}
			cpuReservation, cpuLimit, memoryReservation, memoryLimit := ecs.getContainerLimits(*d.DeployData)
			service.updateServiceLimits(service.clusterName, service.serviceName, cpuReservation, cpuLimit, memoryReservation, memoryLimit)
			time.Sleep(500 * time.Millisecond)
		}
		service.setApiVersion(getApiVersion())
		migrationLogger.Infof("Updated API version to %v", getApiVersion())
	}
	return nil
}
