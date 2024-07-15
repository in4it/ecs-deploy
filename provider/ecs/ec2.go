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
 * CreateSecurityGroup creates a security group
 */
func (e *EC2) CreateSecurityGroup(name, description, vpcID string) (string, error) {
	svc := ec2.New(session.New())

	input := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(name),
		Description: aws.String(description),
		VpcId:       aws.String(vpcID),
	}

	result, err := svc.CreateSecurityGroup(input)
	if err != nil {
		return "", err
	}

	return aws.StringValue(result.GroupId), nil
}

/*
 * CreateSecurityGroup creates a security group
 */
func (e *EC2) CreateSecurityGroupIngressRule(groupId string, FromPort int64, toPort int64, protocol string, sourceSecurityGroupName string, ipRange string) error {
	svc := ec2.New(session.New())

	input := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:    aws.String(groupId),
		FromPort:   aws.Int64(FromPort),
		ToPort:     aws.Int64(toPort),
		IpProtocol: aws.String(protocol),
	}

	if sourceSecurityGroupName != "" {
		input.SourceSecurityGroupName = aws.String(sourceSecurityGroupName)
	}

	if ipRange != "" {
		input.CidrIp = aws.String(ipRange)
	}

	_, err := svc.AuthorizeSecurityGroupIngress(input)
	if err != nil {
		return fmt.Errorf("authorize security group ingress error: %s", err)
	}

	return nil
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
