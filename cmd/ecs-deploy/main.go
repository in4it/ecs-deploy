package main

import (
	"github.com/in4it/ecs-deploy/api"
	"github.com/in4it/ecs-deploy/provider/ecs"
	"github.com/in4it/ecs-deploy/util"
	"github.com/juju/loggo"
	"github.com/spf13/pflag"

	"fmt"
	"os"
)

func startup_checks() {
	mandatoryEnvVars := []string{
		"AWS_REGION",
		"JWT_SECRET",
		"DEPLOY_PASSWORD",
	}
	paramstore := ecs.Paramstore{}
	err := paramstore.RetrieveKeys()
	if err != nil {
		fmt.Printf("Couldn't retrieve variables from parameter store\n")
		os.Exit(1)
	}
	for _, envVar := range mandatoryEnvVars {
		if !util.EnvExists(envVar) {
			fmt.Printf("Environment variable missing: %v\n", envVar)
			os.Exit(1)
		}
	}
	// start controller, check database and pick up any remaining work
	controller := api.Controller{}
	err = controller.Resume()
	if err != nil {
		fmt.Printf("Couldn't start controller: %v\n", err.Error())
		os.Exit(1)
	}
}

func addFlags(f *api.Flags, fs *pflag.FlagSet) {
	fs.BoolVar(&f.Bootstrap, "bootstrap", f.Bootstrap, "bootstrap ECS cluster")
	fs.StringVar(&f.Profile, "profile", f.Profile, "AWS Profile")
	fs.StringVar(&f.Region, "region", f.Region, "AWS Region")
	fs.StringVar(&f.ClusterName, "cluster-name", f.ClusterName, "name of the cluster")
	fs.StringVar(&f.Environment, "environment", f.Environment, "environment (dev/test/staging/uat/prod)")
	fs.StringVar(&f.AlbSecurityGroups, "alb-security-groups", f.AlbSecurityGroups, "security groups to attach to the Application Load Balancer")
	fs.StringVar(&f.EcsSubnets, "ecs-subnets", f.EcsSubnets, "subnets to use for AWS ECS")
	fs.StringVar(&f.CloudwatchLogsPrefix, "cloudwatch-logs-prefix", f.CloudwatchLogsPrefix, "prefix for cloudwatch logs (e.g. mycompany)")
	fs.BoolVar(&f.CloudwatchLogsEnabled, "cloudwatch-logs-enabled", f.CloudwatchLogsEnabled, "enable cloudwatch logs")
	fs.StringVar(&f.KeyName, "key-name", f.KeyName, "ssh key name")
	fs.StringVar(&f.InstanceType, "instance-type", f.InstanceType, "AWS instance type (e.g. t2.micro)")
	fs.StringVar(&f.EcsSecurityGroups, "ecs-security-groups", f.EcsSecurityGroups, "ECS security groups to use")
	fs.StringVar(&f.EcsMinSize, "ecs-min-size", f.EcsMinSize, "ECS minimal size")
	fs.StringVar(&f.EcsMaxSize, "ecs-max-size", f.EcsMaxSize, "ECS maxium size")
	fs.StringVar(&f.EcsDesiredSize, "ecs-desired-size", f.EcsDesiredSize, "ECS desired size")
	fs.BoolVar(&f.ParamstoreEnabled, "paramstore-enabled", f.ParamstoreEnabled, "enable AWS paramater store")
	fs.StringVar(&f.ParamstoreKmsArn, "paramstore-kms-arn", f.ParamstoreKmsArn, "AWS parameter store KMS key ARN")
	fs.StringVar(&f.ParamstorePrefix, "paramstore-prefix", f.ParamstorePrefix, "AWS parameter store prefix (e.g. mycompany)")
	fs.StringVar(&f.LoadbalancerDomain, "loadbalancer-domain", f.LoadbalancerDomain, "domain to access ECS cluster")
	fs.BoolVar(&f.Server, "server", f.Server, "start server")
	fs.StringVar(&f.DeleteCluster, "delete-cluster", f.DeleteCluster, "delete-cluster <cluster name>")
	fs.BoolVar(&f.DisableEcsDeploy, "disable-ecs-deploy", f.DisableEcsDeploy, "disable ecs deploy during bootstrap")
	fs.MarkHidden("disable-ecs-deploy")
}

// @title ecs-deploy
// @version 0.0.1
// @description ecs-deploy is the glue between your CI and ECS. It automates deploys based a simple JSON file Edit
// @contact.name Edward Viaene
// @contact.url	https://github.com/in4it/ecs-deploy
// @contact.email	ward@in4it.io
// license.name	Apache 2.0
func main() {
	// set logging to debug
	if util.GetEnv("DEBUG", "") == "true" {
		loggo.ConfigureLoggers(`<root>=DEBUG`)
	} else {
		loggo.ConfigureLoggers(`<root>=INFO`)
	}

	// parse flags
	flags := api.NewFlags()
	addFlags(flags, pflag.CommandLine)
	pflag.Parse()

	// examine flags
	if flags.Profile != "" {
		os.Setenv("AWS_PROFILE", flags.Profile)
	}
	if flags.Profile != "" {
		os.Setenv("AWS_REGION", flags.Region)
	}
	if flags.Bootstrap {
		if ok, _ := util.AskForConfirmation("Bootstrap ECS Cluster?"); ok {
			controller := api.Controller{}
			err := controller.Bootstrap(flags)
			if err != nil {
				fmt.Printf("Error: %v\n", err.Error())
			}
		}
	} else if flags.DeleteCluster != "" {
		if ok, _ := util.AskForConfirmation("This will delete cluster " + flags.DeleteCluster); ok {
			controller := api.Controller{}
			flags.ClusterName = flags.DeleteCluster
			err := controller.DeleteCluster(flags)
			if err != nil {
				fmt.Printf("Error: %v\n", err.Error())
			}
		}
	} else if flags.Server {
		// startup checks
		startup_checks()

		// Launch API
		api := api.API{}
		err := api.Launch()
		if err != nil {
			panic(err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		pflag.PrintDefaults()
	}
	os.Exit(0)
}
