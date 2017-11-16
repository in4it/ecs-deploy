package main

import (
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/awserr"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/elbv2"
  "github.com/juju/loggo"

  "errors"
  "strconv"
)

// logging
var albLogger = loggo.GetLogger("ecs")

// ALB struct
type ALB struct {
  loadBalancerName string
  loadBalancerArn string
  vpcId string
  listenerArns []string
}


func (a *ALB) init(loadBalancerName string) (error) {
  a.loadBalancerName = loadBalancerName
  // retrieve vpcId and loadBalancerArn
  svc := elbv2.New(session.New())
  input := &elbv2.DescribeLoadBalancersInput{
      Names: []*string{
          aws.String(loadBalancerName),
      },
  }

  result, err := svc.DescribeLoadBalancers(input)
  if err != nil {
      if aerr, ok := err.(awserr.Error); ok {
          switch aerr.Code() {
          case elbv2.ErrCodeLoadBalancerNotFoundException:
            albLogger.Errorf(elbv2.ErrCodeLoadBalancerNotFoundException + ": %v", aerr.Error())
          default:
            albLogger.Errorf(aerr.Error())
          }
      } else {
          // Print the error, cast err to awserr.Error to get the Code and
          // Message from an error.
          albLogger.Errorf(err.Error())
      }
      return errors.New("Could not describe loadbalancer")
  } else if len(result.LoadBalancers) == 0 {
    return errors.New("Could not describe loadbalancer (no elements returned)")
  }
  a.loadBalancerArn = *result.LoadBalancers[0].LoadBalancerArn
  a.loadBalancerName = *result.LoadBalancers[0].LoadBalancerName
  a.vpcId = *result.LoadBalancers[0].VpcId

  // get listeners
  err = a.getListeners()
  if err != nil {
    return err
  } else if len(result.LoadBalancers) == 0 {
    return errors.New("Could not get listeners for loadbalancer (no elements returned)")
  }

  return nil
}

// get the listeners for the loadbalancer
func (a *ALB) getListeners() (error) {
  svc := elbv2.New(session.New())
  input := &elbv2.DescribeListenersInput{ LoadBalancerArn: aws.String(a.loadBalancerArn) }

  result, err := svc.DescribeListeners(input)
  if err != nil {
      if aerr, ok := err.(awserr.Error); ok {
          switch aerr.Code() {
          case elbv2.ErrCodeListenerNotFoundException:
              albLogger.Errorf(elbv2.ErrCodeListenerNotFoundException + ": %v", aerr.Error())
          case elbv2.ErrCodeLoadBalancerNotFoundException:
              albLogger.Errorf(elbv2.ErrCodeLoadBalancerNotFoundException + ": %v", aerr.Error())
          default:
              albLogger.Errorf(aerr.Error())
          }
      } else {
          albLogger.Errorf(err.Error())
      }
      return errors.New("Could not get Listeners for loadbalancer")
  }
  for _, l := range result.Listeners {
    a.listenerArns = append(a.listenerArns, *l.ListenerArn)
  }
  return nil
}

func (a *ALB) createTargetGroup(serviceName string, d Deploy) (*string, error) {
  svc := elbv2.New(session.New())
  input := &elbv2.CreateTargetGroupInput{
      Name:     aws.String(serviceName),
      VpcId:    aws.String(a.vpcId),
      Port:     aws.Int64(d.ServicePort),
      Protocol: aws.String(d.ServiceProtocol),
  }
  if d.HealthCheck.HealthyThreshold != 0 {
    input.SetHealthyThresholdCount(*aws.Int64(d.HealthCheck.HealthyThreshold))
  }
  if d.HealthCheck.UnhealthyThreshold != 0 {
    input.SetUnhealthyThresholdCount(*aws.Int64(d.HealthCheck.UnhealthyThreshold))
  }
  if d.HealthCheck.Path != "" {
    input.SetHealthCheckPath(*aws.String(d.HealthCheck.Path))
  }
  if d.HealthCheck.Port != "" {
    input.SetHealthCheckPort(*aws.String(d.HealthCheck.Port))
  }
  if d.HealthCheck.Protocol != "" {
    input.SetHealthCheckProtocol(*aws.String(d.HealthCheck.Protocol))
  }
  if d.HealthCheck.Interval != 0 {
    input.SetHealthCheckIntervalSeconds(*aws.Int64(d.HealthCheck.Interval))
  }
  if d.HealthCheck.Matcher != "" {
    input.SetMatcher(&elbv2.Matcher{HttpCode: aws.String(d.HealthCheck.Matcher)})
  }

  result, err := svc.CreateTargetGroup(input)
  if err != nil {
      if aerr, ok := err.(awserr.Error); ok {
          switch aerr.Code() {
          case elbv2.ErrCodeDuplicateTargetGroupNameException:
              albLogger.Errorf(elbv2.ErrCodeDuplicateTargetGroupNameException + ": %v", aerr.Error())
          case elbv2.ErrCodeTooManyTargetGroupsException:
              albLogger.Errorf(elbv2.ErrCodeTooManyTargetGroupsException + ": %v", aerr.Error())
          case elbv2.ErrCodeInvalidConfigurationRequestException:
              albLogger.Errorf(elbv2.ErrCodeInvalidConfigurationRequestException + ": %v", aerr.Error())
          default:
              albLogger.Errorf(aerr.Error())
          }
      } else {
          // Print the error, cast err to awserr.Error to get the Code and
          // Message from an error.
          albLogger.Errorf(err.Error())
      }
      return nil, errors.New("Could not create target group")
  } else if len(result.TargetGroups) == 0 {
      return nil, errors.New("Could not create target group (target group list is empty)")
  }
  return result.TargetGroups[0].TargetGroupArn, nil
}

func (a *ALB) getHighestRule() (int64, error) {
  var highest int64
  svc := elbv2.New(session.New())

  input := &elbv2.DescribeRulesInput{ ListenerArn: aws.String(a.listenerArns[0]) }

  c := true // parse more pages if c is true
  result, err := svc.DescribeRules(input)
  for c {
    if err != nil {
        if aerr, ok := err.(awserr.Error); ok {
            switch aerr.Code() {
            case elbv2.ErrCodeListenerNotFoundException:
                albLogger.Errorf(elbv2.ErrCodeListenerNotFoundException + ": %v", aerr.Error())
            case elbv2.ErrCodeRuleNotFoundException:
                albLogger.Errorf(elbv2.ErrCodeRuleNotFoundException + ": %v", aerr.Error())
            default:
                albLogger.Errorf(aerr.Error())
            }
        } else {
            // Print the error, cast err to awserr.Error to get the Code and
            // Message from an error.
            albLogger.Errorf(err.Error())
        }
        return 0, errors.New("Could not describe alb listener rules")
    }


    albLogger.Debugf("Looping rules: %+v", result.Rules)
    for _, rule := range result.Rules {
      if i, _ := strconv.ParseInt(*rule.Priority, 10, 64) ; i > highest {
        albLogger.Debugf("Found rule with priority: %d", i)
        highest = i
      }
    }
    if result.NextMarker == nil || len(*result.NextMarker) == 0 {
      c = false
    } else {
      input.SetMarker(*result.NextMarker)
      result, err = svc.DescribeRules(input)
    }
  }

  albLogger.Debugf("Higest rule: %d", highest)

  return highest, nil
}

func (a *ALB) createRuleForAllListeners(targetGroupArn string, rule string, priority int64) (error) {
  for _, l := range a.listenerArns {
    err := a.createRule(l, targetGroupArn, rule, priority)
    if err != nil {
      return err
    }
  }
  return nil
}

func (a *ALB) createRule(listenerArn string, targetGroupArn string, rule string, priority int64) (error) {
  svc := elbv2.New(session.New())
  input := &elbv2.CreateRuleInput{
      Actions: []*elbv2.Action{
          {
              TargetGroupArn: aws.String(targetGroupArn),
              Type:           aws.String("forward"),
          },
      },
      Conditions: []*elbv2.RuleCondition{
          {
              Field: aws.String("path-pattern"),
              Values: []*string{
                  aws.String(rule),
              },
          },
      },
      ListenerArn: aws.String(listenerArn),
      Priority:    aws.Int64(priority),
  }

  _, err := svc.CreateRule(input)
  if err != nil {
      if aerr, ok := err.(awserr.Error); ok {
          switch aerr.Code() {
          case elbv2.ErrCodePriorityInUseException:
              albLogger.Errorf(elbv2.ErrCodePriorityInUseException + ": %v", aerr.Error())
          case elbv2.ErrCodeTooManyTargetGroupsException:
              albLogger.Errorf(elbv2.ErrCodeTooManyTargetGroupsException + ": %v", aerr.Error())
          case elbv2.ErrCodeTooManyRulesException:
              albLogger.Errorf(elbv2.ErrCodeTooManyRulesException + ": %v", aerr.Error())
          case elbv2.ErrCodeTargetGroupAssociationLimitException:
              albLogger.Errorf(elbv2.ErrCodeTargetGroupAssociationLimitException + ": %v", aerr.Error())
          case elbv2.ErrCodeIncompatibleProtocolsException:
              albLogger.Errorf(elbv2.ErrCodeIncompatibleProtocolsException + ": %v", aerr.Error())
          case elbv2.ErrCodeListenerNotFoundException:
              albLogger.Errorf(elbv2.ErrCodeListenerNotFoundException + ": %v", aerr.Error())
          case elbv2.ErrCodeTargetGroupNotFoundException:
              albLogger.Errorf(elbv2.ErrCodeTargetGroupNotFoundException + ": %v", aerr.Error())
          case elbv2.ErrCodeInvalidConfigurationRequestException:
              albLogger.Errorf(elbv2.ErrCodeInvalidConfigurationRequestException + ": %v", aerr.Error())
          case elbv2.ErrCodeTooManyRegistrationsForTargetIdException:
              albLogger.Errorf(elbv2.ErrCodeTooManyRegistrationsForTargetIdException + ": %v", aerr.Error())
          case elbv2.ErrCodeTooManyTargetsException:
              albLogger.Errorf(elbv2.ErrCodeTooManyTargetsException + ": %v", aerr.Error())
          default:
              albLogger.Errorf(aerr.Error())
          }
      } else {
          // Print the error, cast err to awserr.Error to get the Code and
          // Message from an error.
          albLogger.Errorf(err.Error())
      }
      return errors.New("Could not create alb rule")
  }
  return nil
}
