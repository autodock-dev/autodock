package cfntemplate

import (
	"fmt"
	"log"

	"autodock/utils"

	gocfn "github.com/awslabs/goformation/v7/cloudformation"
	"github.com/awslabs/goformation/v7/cloudformation/certificatemanager"
	"github.com/awslabs/goformation/v7/cloudformation/ec2"
	"github.com/awslabs/goformation/v7/cloudformation/ecr"
	"github.com/awslabs/goformation/v7/cloudformation/route53"
	"github.com/compose-spec/compose-go/v2/types"
)

// represent a set of strings
type StringMapSet map[string]struct{}

// Generate a "bootstrap" template, which contains common resources for the services defined in the Compose file
// The compose file is parsed as a Compose Project
func GenerateBootstrapTemplate(project *types.Project) string {
	template := gocfn.NewTemplate()

	// A set of root domains
	rootDomains := make(StringMapSet)
	for _, service := range project.Services {
		if service.Extensions["x-domain-name"] == nil {
			log.Printf("[warn] Missing or empty x-domain-name field in service %s", service.Name)
			continue
		}
		domainName := fmt.Sprint(service.Extensions["x-domain-name"])
		rootDomain := utils.GetRootDomain(domainName)
		if _, ok := rootDomains[rootDomain]; !ok {
			rootDomains[rootDomain] = struct{}{}
		}
	}

	for rootDomain := range rootDomains {
		hostedZoneResourceName := fmt.Sprintf("%sHostedZone", utils.ToAlphabel(rootDomain))
		template.Resources[hostedZoneResourceName] = &route53.HostedZone{
			Name: gocfn.String(rootDomain),
			HostedZoneConfig: &route53.HostedZone_HostedZoneConfig{
				Comment: gocfn.String("DNS config for " + rootDomain),
			},
		}

		certificateResourceName := fmt.Sprintf("%sCertificate", utils.ToAlphabel(rootDomain))
		template.Resources[certificateResourceName] = &certificatemanager.Certificate{
			DomainName: rootDomain,
			SubjectAlternativeNames: []string{
				fmt.Sprintf("*.%s", rootDomain),
			},
			ValidationMethod: gocfn.String("DNS"), // Recommended for Route53 domain

			DomainValidationOptions: []certificatemanager.Certificate_DomainValidationOption{
				{
					DomainName:   rootDomain,
					HostedZoneId: gocfn.String(gocfn.Ref(hostedZoneResourceName)),
				},
			},
		}
	}

	vpcName := fmt.Sprintf("%sVPC", project.Name)
	template.Resources[vpcName] = &ec2.VPC{
		CidrBlock:          gocfn.String("10.0.0.0/16"),
		EnableDnsSupport:   gocfn.Bool(true),
		EnableDnsHostnames: gocfn.Bool(true),
	}

	privateSubnetName1 := fmt.Sprintf("%sPrivateSubnet1", project.Name)
	template.Resources[privateSubnetName1] = &ec2.Subnet{
		VpcId:            gocfn.Ref(vpcName),
		CidrBlock:        gocfn.String("10.0.1.0/24"),
		AvailabilityZone: gocfn.String(gocfn.Select(0, gocfn.GetAZs(""))),
	}

	// 2 private subnets in different AZs to improve availability
	privateSubnetName2 := fmt.Sprintf("%sPrivateSubnet2", project.Name)
	template.Resources[privateSubnetName2] = &ec2.Subnet{
		VpcId:            gocfn.Ref(vpcName),
		CidrBlock:        gocfn.String("10.0.2.0/24"),
		AvailabilityZone: gocfn.String(gocfn.Select(1, gocfn.GetAZs(""))),
	}

	// associate the subnets with a route table
	privateRouteTableName := fmt.Sprintf("%sPrivateRouteTable", project.Name)
	template.Resources[privateRouteTableName] = &ec2.RouteTable{
		VpcId: gocfn.Ref(vpcName),
	}
	template.Resources["PrivateSubnet1RouteTableAssoc"] = &ec2.SubnetRouteTableAssociation{
		SubnetId:     gocfn.Ref(privateSubnetName1),
		RouteTableId: gocfn.Ref(privateRouteTableName),
	}
	template.Resources["PrivateSubnet2RouteTableAssoc"] = &ec2.SubnetRouteTableAssociation{
		SubnetId:     gocfn.Ref(privateSubnetName2),
		RouteTableId: gocfn.Ref(privateRouteTableName),
	}

	// internet gateway
	template.Resources["InternetGateway"] = &ec2.InternetGateway{}
	template.Resources["InternetGatewayAttachment"] = &ec2.VPCGatewayAttachment{
		VpcId:             gocfn.Ref(vpcName),
		InternetGatewayId: gocfn.String(gocfn.Ref("InternetGateway")),
	}

	// public subnets
	publicSubnetName1 := fmt.Sprintf("%sPublicSubnet1", project.Name)
	template.Resources[publicSubnetName1] = &ec2.Subnet{
		VpcId:            gocfn.Ref(vpcName),
		CidrBlock:        gocfn.String("10.0.3.0/24"),
		AvailabilityZone: gocfn.String(gocfn.Select(0, gocfn.GetAZs(""))),
	}

	publicSubnetName2 := fmt.Sprintf("%sPublicSubnet2", project.Name)
	template.Resources[publicSubnetName2] = &ec2.Subnet{
		VpcId:            gocfn.Ref(vpcName),
		CidrBlock:        gocfn.String("10.0.4.0/24"),
		AvailabilityZone: gocfn.String(gocfn.Select(1, gocfn.GetAZs(""))),
	}

	// public route table
	publicRouteTableName := fmt.Sprintf("%sPublicRouteTable", project.Name)
	template.Resources[publicRouteTableName] = &ec2.RouteTable{
		VpcId: gocfn.Ref(vpcName),
	}
	template.Resources["PublicRoute"] = &ec2.Route{
		RouteTableId:         gocfn.Ref(publicRouteTableName),
		DestinationCidrBlock: gocfn.String("0.0.0.0/0"), // Send all external traffic to the Internet Gateway
		GatewayId:            gocfn.String(gocfn.Ref("InternetGateway")),
	}
	template.Resources["PublicSubnet1RouteTableAssoc"] = &ec2.SubnetRouteTableAssociation{
		SubnetId:     gocfn.Ref(publicSubnetName1),
		RouteTableId: gocfn.Ref(publicRouteTableName),
	}
	template.Resources["PublicSubnet2RouteTableAssoc"] = &ec2.SubnetRouteTableAssociation{
		SubnetId:     gocfn.Ref(publicSubnetName2),
		RouteTableId: gocfn.Ref(publicRouteTableName),
	}

	// security groups
	// for alb
	albSecGroupName := "AlbSecurityGroup"
	template.Resources[albSecGroupName] = &ec2.SecurityGroup{
		GroupDescription: "For ALB",
		VpcId:            gocfn.String(gocfn.Ref(vpcName)),
		SecurityGroupIngress: []ec2.SecurityGroup_Ingress{
			{
				IpProtocol:  "tcp",
				FromPort:    gocfn.Int(443),
				ToPort:      gocfn.Int(443),
				CidrIp:      gocfn.String("0.0.0.0/0"),
				Description: gocfn.String("Allow HTTPS to from anywhere"),
			},
			{
				IpProtocol:  "tcp",
				FromPort:    gocfn.Int(80),
				ToPort:      gocfn.Int(80),
				CidrIp:      gocfn.String("0.0.0.0/0"),
				Description: gocfn.String("Allow HTTP to from anywhere"),
			},
		},
	}
	// for fargate tasks
	fargateTaskSecGroupName := "FargateTaskSecurityGroup"
	template.Resources[fargateTaskSecGroupName] = &ec2.SecurityGroup{
		GroupDescription: "For Fargate tasks",
		VpcId:            gocfn.String(gocfn.Ref(vpcName)),
		SecurityGroupIngress: []ec2.SecurityGroup_Ingress{
			{
				IpProtocol:            "tcp",
				FromPort:              gocfn.Int(3000), // TODO: get the ports from compose file
				ToPort:                gocfn.Int(3000),
				SourceSecurityGroupId: gocfn.String(gocfn.Ref(albSecGroupName)),
				Description:           gocfn.String("Allow traffic from ALB"),
			},
		},
	}
	// for vpc endpoints
	vpeSecGroupName := "VpcEndpointSecurityGroup"
	template.Resources[vpeSecGroupName] = &ec2.SecurityGroup{
		GroupDescription: "For VPC Ednpoints",
		VpcId:            gocfn.String(gocfn.Ref(vpcName)),
	}
	template.Resources["VpcEndpointSecurityGroupIngress"] = &ec2.SecurityGroupIngress{
		GroupId:               gocfn.String(gocfn.Ref(vpeSecGroupName)),
		IpProtocol:            "tcp",
		FromPort:              gocfn.Int(443),
		ToPort:                gocfn.Int(443),
		SourceSecurityGroupId: gocfn.String(gocfn.Ref(fargateTaskSecGroupName)),
		Description:           gocfn.String("Allow HTTPS to from Fargate tasks"),
	}

	// Vpc endpoints
	// For ECR API
	template.Resources["EcrApiVpcEndpoint"] = &ec2.VPCEndpoint{
		VpcId:           gocfn.Ref(vpcName),
		ServiceName:     gocfn.Sub("com.amazonaws.${AWS::Region}.ecr.api"),
		VpcEndpointType: gocfn.String("Interface"),
		SubnetIds: []string{
			gocfn.Ref(privateSubnetName1),
			gocfn.Ref(privateSubnetName2),
		},
		SecurityGroupIds: []string{
			gocfn.Ref(vpeSecGroupName),
		},
		PrivateDnsEnabled: gocfn.Bool(true),
	}
	// For ECR DKR
	template.Resources["EcrDkrVpcEndpoint"] = &ec2.VPCEndpoint{
		VpcId:           gocfn.Ref(vpcName),
		ServiceName:     gocfn.Sub("com.amazonaws.${AWS::Region}.ecr.dkr"),
		VpcEndpointType: gocfn.String("Interface"),
		SubnetIds: []string{
			gocfn.Ref(privateSubnetName1),
			gocfn.Ref(privateSubnetName2),
		},
		SecurityGroupIds: []string{
			gocfn.Ref(vpeSecGroupName),
		},
		PrivateDnsEnabled: gocfn.Bool(true),
	}
	// For CloudWatch
	template.Resources["CloudWatchVpcEndpoint"] = &ec2.VPCEndpoint{
		VpcId:           gocfn.Ref(vpcName),
		ServiceName:     gocfn.Sub("com.amazonaws.${AWS::Region}.logs"),
		VpcEndpointType: gocfn.String("Interface"),
		SubnetIds: []string{
			gocfn.Ref(privateSubnetName1),
			gocfn.Ref(privateSubnetName2),
		},
		SecurityGroupIds: []string{
			gocfn.Ref(vpeSecGroupName),
		},
		PrivateDnsEnabled: gocfn.Bool(true),
	}
	// VPC Gateway Endpoint for S3 (required by ECR)
	template.Resources["S3GatewayVpcEndpoint"] = &ec2.VPCEndpoint{
		VpcId:           gocfn.Ref(vpcName),
		ServiceName:     gocfn.Sub("com.amazonaws.${AWS::Region}.s3"),
		VpcEndpointType: gocfn.String("Gateway"),
		RouteTableIds: []string{
			gocfn.Ref(privateRouteTableName),
		},
	}

	// create ECR repositories for each service
	for _, service := range project.Services {
		if service.Name == "client" {
			template.Resources[fmt.Sprintf("ImageRepositoryFor%s", service.Name)] = &ecr.Repository{
				RepositoryName: gocfn.String(service.Image),
			}
		}
	}

	template.Outputs["PrivateSubnet1"] = gocfn.Output{
		Value: gocfn.Ref(privateSubnetName1),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sPrivateSubnet1", project.Name),
		},
	}
	template.Outputs["PrivateSubnet2"] = gocfn.Output{
		Value: gocfn.Ref(privateSubnetName2),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sPrivateSubnet2", project.Name),
		},
	}
	template.Outputs["FargateTaskSecurityGroup"] = gocfn.Output{
		Value: gocfn.Ref(fargateTaskSecGroupName),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sFargateTaskSecurityGroup", project.Name),
		},
	}
	template.Outputs["AlbSecurityGroup"] = gocfn.Output{
		Value: gocfn.Ref(albSecGroupName),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sAlbSecurityGroup", project.Name),
		},
	}
	template.Outputs["PublicSubnet1"] = gocfn.Output{
		Value: gocfn.Ref(publicSubnetName1),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sPublicSubnet1", project.Name),
		},
	}
	template.Outputs["PublicSubnet2"] = gocfn.Output{
		Value: gocfn.Ref(publicSubnetName2),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sPublicSubnet2", project.Name),
		},
	}
	template.Outputs["VpcId"] = gocfn.Output{
		Value: gocfn.Ref(vpcName),
		Export: &gocfn.Export{
			Name: fmt.Sprintf("%sVpcId", project.Name),
		},
	}
	for rootDomain := range rootDomains {
		template.Outputs[fmt.Sprintf("%sHostedZone", utils.ToAlphabel(rootDomain))] = gocfn.Output{
			Value: gocfn.Ref(fmt.Sprintf("%sHostedZone", utils.ToAlphabel(rootDomain))),
			Export: &gocfn.Export{
				Name: fmt.Sprintf("%sHostedZone", utils.ToAlphabel(rootDomain)),
			},
		}
		template.Outputs[fmt.Sprintf("%sCertificate", utils.ToAlphabel(rootDomain))] = gocfn.Output{
			Value: gocfn.Ref(fmt.Sprintf("%sCertificate", utils.ToAlphabel(rootDomain))),
			Export: &gocfn.Export{
				Name: fmt.Sprintf("%sCertificate", utils.ToAlphabel(rootDomain)),
			},
		}
	}
	yml, err := template.YAML()
	if err != nil {
		log.Fatalf("Failed to generate YAML from a cloudformation template for bootstrapping: %v", err)
		return ""
	} else {
		log.Printf("[debug] [stack %s]: Generated bootstrap CloudFormation template:\n %s\n", project.Name, string(yml))
		return string(yml)
	}
}
