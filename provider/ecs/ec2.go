package ecs

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/juju/loggo"

	"fmt"
)

// logging
var ec2Logger = loggo.GetLogger("ec2")

// EC2 struct
type EC2 struct {
}

/*
 * GetSecurityGroupID retrieves the id from the security group based on the name
 */
func (e *EC2) GetSecurityGroupID(name string) (string, error) {
	svc := ec2.New(session.New())

	input := &ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("group-name"),
				Values: aws.StringSlice([]string{name}),
			},
		},
	}

	result, err := svc.DescribeSecurityGroups(input)
	if err != nil {
		return "", err
	}

	if len(result.SecurityGroups) == 0 {
		return "", fmt.Errorf("No security groups returned")
	}

	return aws.StringValue(result.SecurityGroups[0].GroupId), nil
}

/*
 * GetSubnetId retrieves the id from the subnet based on the name
 */
func (e *EC2) GetSubnetID(name string) (string, error) {
	svc := ec2.New(session.New())

	input := &ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: aws.StringSlice([]string{name}),
			},
		},
	}

	result, err := svc.DescribeSubnets(input)
	if err != nil {
		return "", err
	}

	if len(result.Subnets) == 0 {
		return "", fmt.Errorf("No subnets returned")
	}

	return aws.StringValue(result.Subnets[0].SubnetId), nil
}
