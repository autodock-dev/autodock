package cfntemplate

import (
	"autodock/utils"
	"fmt"
	"log"

	gocfn "github.com/awslabs/goformation/v7/cloudformation"
	"github.com/awslabs/goformation/v7/cloudformation/ecs"
	elbv2 "github.com/awslabs/goformation/v7/cloudformation/elasticloadbalancingv2"
	"github.com/awslabs/goformation/v7/cloudformation/iam"
	"github.com/awslabs/goformation/v7/cloudformation/logs"
	"github.com/awslabs/goformation/v7/cloudformation/route53"
	"github.com/compose-spec/compose-go/v2/types"
)

// Generate Cloudformation templates for a service defined in the Compose file
func GenerateServiceTemplate(project *types.Project, service *types.ServiceConfig, imageTag string) string {

	/**
	* [ ] Add network configuration to the ECS service
	* [ ] Deployment testing
	* [ ] Add load balancer
	* [ ] add dns record to load balancer
	 */
	template := gocfn.NewTemplate()

	taskLogGroupName := fmt.Sprintf("ecs/%s-%s", service.Name, service.ContainerName)
	taskLogGroupResourceName := fmt.Sprintf("%sEcsTaskLogGroup", service.Name)
	template.Resources[taskLogGroupResourceName] = &logs.LogGroup{
		LogGroupName: gocfn.String(taskLogGroupName),
	}

	clusterResourceName := fmt.Sprintf("%sEcsFargateCluster", service.Name)
	template.Resources[clusterResourceName] = &ecs.Cluster{
		ClusterName: gocfn.String(fmt.Sprintf("%sCluster", service.Name)),
	}
	containerDefinition := ecs.TaskDefinition_ContainerDefinition{
		Name:  service.ContainerName,
		Image: imageTag,
		PortMappings: []ecs.TaskDefinition_PortMapping{
			{
				ContainerPort: gocfn.Int(3000), // TODO: get the port from the compose file
				Protocol:      gocfn.String("tcp"),
			},
		},
		LogConfiguration: &ecs.TaskDefinition_LogConfiguration{
			LogDriver: "awslogs",
			Options: map[string]string{
				"awslogs-group":         gocfn.Ref(taskLogGroupResourceName),
				"awslogs-region":        gocfn.Ref("AWS::Region"),
				"awslogs-stream-prefix": gocfn.Ref("AWS::StackName"),
			},
		},
	}

	taskExecutionRoleResourceName := fmt.Sprintf("%sEcsTaskExecutionRole", service.Name)
	template.Resources[taskExecutionRoleResourceName] = &iam.Role{
		AssumeRolePolicyDocument: map[string]interface{}{
			"Version": "2012-10-17",
			"Statement": []map[string]interface{}{
				{
					"Effect": "Allow",
					"Action": "sts:AssumeRole",
					"Principal": map[string]interface{}{
						"Service": "ecs-tasks.amazonaws.com",
					},
				},
			},
		},
		ManagedPolicyArns: []string{
			"arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy",
		},
	}

	taskDefResourceName := fmt.Sprintf("%sEcsTaskDefinition", service.Name)
	template.Resources[taskDefResourceName] = &ecs.TaskDefinition{
		NetworkMode:             gocfn.String("awsvpc"), // required for fargate
		RequiresCompatibilities: []string{"FARGATE"},
		ContainerDefinitions:    []ecs.TaskDefinition_ContainerDefinition{containerDefinition},
		Cpu:                     gocfn.String("1024"),
		Memory:                  gocfn.String("2048"),
		ExecutionRoleArn:        gocfn.String(gocfn.Ref(taskExecutionRoleResourceName)),
		RuntimePlatform: &ecs.TaskDefinition_RuntimePlatform{
			CpuArchitecture:       gocfn.String("ARM64"),
			OperatingSystemFamily: gocfn.String("LINUX"),
		},
	}

	// ALB
	albResourceName := fmt.Sprintf("%sAlb", service.Name)
	template.Resources[albResourceName] = &elbv2.LoadBalancer{
		Name:   gocfn.String(fmt.Sprintf("%sAlb", service.Name)),
		Scheme: gocfn.String("internet-facing"),
		Subnets: []string{
			gocfn.ImportValue(fmt.Sprintf("%sPublicSubnet1", project.Name)),
			gocfn.ImportValue(fmt.Sprintf("%sPublicSubnet2", project.Name)),
		},
		SecurityGroups: []string{
			gocfn.ImportValue(fmt.Sprintf("%sAlbSecurityGroup", project.Name)),
		},
		Type: gocfn.String("application"), // Specify it's a Application Load Balancer
	}

	// Domain name record set
	domainName := fmt.Sprint(service.Extensions["x-domain-name"])
	if domainName == "" {
		log.Fatalf("Empty domain name")
	}
	rootDomain := utils.GetRootDomain(domainName)
	recordSetResourceName := fmt.Sprintf("%sRecordSet", service.Name)
	template.Resources[recordSetResourceName] = &route53.RecordSet{
		Name:         domainName + ".",
		HostedZoneId: gocfn.String(gocfn.ImportValue(fmt.Sprintf("%sHostedZone", utils.ToAlphabel(rootDomain)))),
		Type:         "A",
		AliasTarget: &route53.RecordSet_AliasTarget{
			DNSName:      gocfn.GetAtt(albResourceName, "DNSName"),
			HostedZoneId: gocfn.GetAtt(albResourceName, "CanonicalHostedZoneID"),
			// EvaluateTargetHealth: gocfn.Bool(true),
		},
	}

	// ALB target group
	albTargetGroupResourceName := fmt.Sprintf("%sAlbTargetGroup", service.Name)
	template.Resources[albTargetGroupResourceName] = &elbv2.TargetGroup{
		Name:       gocfn.String(fmt.Sprintf("%sAlbTargetGroup", service.Name)),
		Protocol:   gocfn.String("HTTP"),
		Port:       gocfn.Int(80),
		TargetType: gocfn.String("ip"), // required for Fargate
		VpcId:      gocfn.String(gocfn.ImportValue(fmt.Sprintf("%sVpcId", project.Name))),

		HealthCheckIntervalSeconds: gocfn.Int(30),
		HealthCheckPath:            gocfn.String("/"),
		HealthCheckPort:            gocfn.String("3000"),
		HealthCheckProtocol:        gocfn.String("HTTP"),
		HealthCheckTimeoutSeconds:  gocfn.Int(5),
		// HealthCheckEnabled:         gocfn.Bool(true),

		Matcher: &elbv2.TargetGroup_Matcher{HttpCode: gocfn.String("200")},
	}

	httpsListenerResourceName := fmt.Sprintf("%sHttpsListener", service.Name)
	template.Resources[httpsListenerResourceName] = &elbv2.Listener{
		LoadBalancerArn: gocfn.Ref(albResourceName),
		Protocol:        gocfn.String("HTTPS"),
		Port:            gocfn.Int(443),
		DefaultActions: []elbv2.Listener_Action{
			{
				Type:           "forward",
				TargetGroupArn: gocfn.String(gocfn.Ref(albTargetGroupResourceName)),
			},
		},
		AWSCloudFormationDependsOn: []string{
			albTargetGroupResourceName,
			albResourceName,
		},
		Certificates: []elbv2.Listener_Certificate{
			{
				CertificateArn: gocfn.String(gocfn.ImportValue(fmt.Sprintf("%sCertificate", utils.ToAlphabel(rootDomain)))),
			},
		},
		SslPolicy: gocfn.String("ELBSecurityPolicy-2016-08"),
	}

	// redirect to httpsListener
	httpListenerResourceName := fmt.Sprintf("%sHttpListener", service.Name)
	template.Resources[httpListenerResourceName] = &elbv2.Listener{
		LoadBalancerArn: gocfn.Ref(albResourceName),
		Protocol:        gocfn.String("HTTP"),
		Port:            gocfn.Int(80),
		DefaultActions: []elbv2.Listener_Action{
			{
				Type: "redirect",
				RedirectConfig: &elbv2.Listener_RedirectConfig{
					Protocol:   gocfn.String("HTTPS"),
					Port:       gocfn.String("443"),
					StatusCode: "HTTP_301", // permanent redirect
				},
			},
		},
		AWSCloudFormationDependsOn: []string{
			albResourceName,
		},
	}

	// ECS service
	serviceResourceName := fmt.Sprintf("%sEcsFargateService", service.Name)
	template.Resources[serviceResourceName] = &ecs.Service{
		ServiceName:    gocfn.String(fmt.Sprintf("%sFargateService", service.Name)),
		Cluster:        gocfn.String(gocfn.Ref(clusterResourceName)),
		DesiredCount:   gocfn.Int(1),
		LaunchType:     gocfn.String("FARGATE"),
		TaskDefinition: gocfn.String(gocfn.Ref(taskDefResourceName)),
		NetworkConfiguration: &ecs.Service_NetworkConfiguration{
			AwsvpcConfiguration: &ecs.Service_AwsVpcConfiguration{
				Subnets: []string{
					gocfn.ImportValue(fmt.Sprintf("%sPrivateSubnet1", project.Name)),
					gocfn.ImportValue(fmt.Sprintf("%sPrivateSubnet2", project.Name)),
				},
				SecurityGroups: []string{
					gocfn.ImportValue(fmt.Sprintf("%sFargateTaskSecurityGroup", project.Name)),
				},
				AssignPublicIp: gocfn.String("DISABLED"),
			},
		},
		LoadBalancers: []ecs.Service_LoadBalancer{
			{
				ContainerName:  gocfn.String(service.ContainerName),
				ContainerPort:  gocfn.Int(3000), // TODO: get the port from the compose file
				TargetGroupArn: gocfn.String(gocfn.Ref(albTargetGroupResourceName)),
			},
		},
		AWSCloudFormationDependsOn: []string{
			albTargetGroupResourceName,
			albResourceName,
			httpsListenerResourceName,
			httpListenerResourceName,
		},
	}

	yml, err := template.YAML()
	if err != nil {
		panic(fmt.Sprintf("Failed to generate YAML from a cloudformation template for the given Compose stack: %s", err)) // TODO: don't use panic
	} else {
		fmt.Printf("\nGenerated this CloudFormation template for stack %s:\n %s\n", project.Name, string(yml))
		return string(yml)
	}

}
