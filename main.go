package main

import (
	"autodock/aws"
	"autodock/aws/cfntemplate"
	"autodock/compose"
	"autodock/docker"
	"context"
	"fmt"
	"log"
	"os"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/spf13/cobra"
)

const version = "0.0.1"

var composeFile string
var ctx = context.Background()

// Bootstrap the cloud account with required resources needed for deployments, such as a Docker registry"
func bootstrap(project *composeTypes.Project) {
	y := cfntemplate.GenerateBootstrapTemplate(project)
	if err := aws.StackDeploy(ctx, fmt.Sprintf("%s-bootstrap", project.Name), y); err != nil {
		log.Fatalf("[error] Error deploying Bootstrap stack: %s\n", err)
	}
}

// Build Docker images for services in a Compose file
func build(service *composeTypes.ServiceConfig) string {
	imageTag := docker.BuildImage(ctx, service)
	docker.PushImage(ctx, service, imageTag)
	return imageTag
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "autodock",
		Short: "A CLI tool for deploying Docker Compose stacks to AWS",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if composeFile == "" {
				composeFile = "docker-compose.yaml"
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Welcome to autodock version " + version)
			fmt.Println("audodock helps you to deploy your Docker Compose stack to your cloud provider without writing any additional code.")
			fmt.Println("`autodock deploy -f docker-compose.yml")
		},
	}

	rootCmd.PersistentFlags().StringVarP(&composeFile, "file", "f", "docker-compose.yaml", "Path to Docker Compose file")

	deployCmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your Docker Compose stack to AWS",
		Run: func(cmd *cobra.Command, args []string) {
			project := compose.Parse(composeFile)
			bootstrap(project)

			for _, service := range project.Services {
				if service.Name == "client" {
					imageTag := build(&service)
					y := cfntemplate.GenerateServiceTemplate(project, &service, imageTag)
					if y == "" {
						fmt.Println("No template to deploy.")
						return
					}
					if err := aws.StackDeploy(ctx, fmt.Sprintf("%s-%s", project.Name, service.Name), y); err != nil {
						fmt.Printf("Error deploying stack: %s\n", err)
					}
				}
			}
		},
	}

	synthCmd := &cobra.Command{
		Use:   "synth",
		Short: "SynthesizeCloudformation templates from a Compose file",
		Run: func(cmd *cobra.Command, args []string) {
			project := compose.Parse(composeFile)
			bootstrapTemplate := cfntemplate.GenerateBootstrapTemplate(project)
			if err := os.WriteFile("bootstrap-template.yaml", []byte(bootstrapTemplate), 0644); err != nil {
				log.Fatalf("Error writing bootstrap template to file: %s\n", err)
			}

			for _, service := range project.Services {
				if service.Name == "client" {
					imageTag := build(&service)
					serviceTemplate := cfntemplate.GenerateServiceTemplate(project, &service, imageTag)
					if err := os.WriteFile(fmt.Sprintf("%s-service-template.yaml", service.Name), []byte(serviceTemplate), 0644); err != nil {
						log.Fatalf("Error writing service template to file: %s\n", err)
					}
				}
			}
		},
	}

	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap the cloud account with required resources needed for deployments, such as a Docker registry",
		Run: func(cmd *cobra.Command, args []string) {
			project := compose.Parse(composeFile)
			bootstrap(project)
		},
	}

	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(synthCmd)
	rootCmd.AddCommand(bootstrapCmd)

	// TODO: remove this in prod
	randomDevCmd := &cobra.Command{
		Use:   "debug",
		Short: "debugging random stuff",
		Run: func(cmd *cobra.Command, args []string) {
			// check build
			compose.Parse(composeFile)
			// build(project)
		},
	}

	rootCmd.AddCommand(randomDevCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
