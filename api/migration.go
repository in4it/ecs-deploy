package api

import (
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/service"
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
		migrationLogger.Infof("Starting migration from %v to %v", apiVersion, m.getApiVersion())
		var dss service.DynamoServices
		s := service.NewService()
		e := ecs.ECS{}
		err := s.GetServices(&dss)
		if err != nil {
			return err
		}
		for _, ds := range dss.Services {
			// doing one per half second not to overload db
			s.ClusterName = ds.C
			s.ServiceName = ds.S
			d, err := s.GetLastDeploy()
			if err != nil {
				return err
			}
			cpuReservation, cpuLimit, memoryReservation, memoryLimit := e.GetContainerLimits(*d.DeployData)
			s.UpdateServiceLimits(s.ClusterName, s.ServiceName, cpuReservation, cpuLimit, memoryReservation, memoryLimit)
			time.Sleep(500 * time.Millisecond)
		}
		s.SetApiVersion(m.getApiVersion())
		migrationLogger.Infof("Updated API version to %v", m.getApiVersion())
	}
	return nil
}

func (m *Migration) getApiVersion() string {
	return apiVersion
}
