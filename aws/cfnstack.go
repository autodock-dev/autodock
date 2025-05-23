package aws

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cloudformationtypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// Check if a Cfn stack exists
func stackExists(ctx context.Context, cf *cloudformation.Client, stackName string) bool {
	_, err := cf.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})

	if err != nil {
		var notFoundErr *cloudformationtypes.StackNotFoundException
		if errors.As(err, &notFoundErr) || containsIgnoreCase(err.Error(), "Stack with id "+stackName+" does not exist") {
			log.Printf("Stack does not exist because DescribeStacks got error: %s\n", err)
			return false
		} else {
			log.Printf("failed to DescribeStack: %s\n", err)
			return false
		}
	}

	return true
}

// Check a Cfn stack status
func stackStatus(ctx context.Context, cf *cloudformation.Client, stackName string) (string, error) {
	stack, err := cf.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{
		StackName: &stackName,
	})
	if err != nil {
		return "", err
	}
	return string(stack.Stacks[0].StackStatus), nil
}

// Deploys the given CloudFormation YAML template to AWS
func StackDeploy(ctx context.Context, stackName string, templateBody string) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}
	cf := cloudformation.NewFromConfig(cfg)

	stackExists := stackExists(ctx, cf, stackName)

	if stackExists {
		// Try to update the stack
		_, err := cf.UpdateStack(ctx, &cloudformation.UpdateStackInput{
			StackName:    &stackName,
			TemplateBody: &templateBody,
			Capabilities: []cloudformationtypes.Capability{
				cloudformationtypes.CapabilityCapabilityIam,
				cloudformationtypes.CapabilityCapabilityNamedIam,
			},
		})
		if err != nil {
			log.Printf("[debug] Error returned from UpdateStack: %s\n", err)
			// If no updates are to be performed, AWS returns a specific error
			if err.Error() == "No updates are to be performed." || (err.Error() != "" && (containsIgnoreCase(err.Error(), "no updates are to be performed"))) {
				log.Printf("[info] [stack: %s] No updates to perform on the stack.", stackName)
				return nil
			}
			log.Fatalf("[error] [stack: %s] failed to update stack: %v", stackName, err)
		}
		log.Printf("[info] [stack: %s] Stack update initiated. Check AWS CloudFormation console for progress.", stackName)
	} else {
		// Stack does not exist, create it
		_, err := cf.CreateStack(ctx, &cloudformation.CreateStackInput{
			StackName:    &stackName,
			TemplateBody: &templateBody,
			Capabilities: []cloudformationtypes.Capability{
				cloudformationtypes.CapabilityCapabilityIam,
				cloudformationtypes.CapabilityCapabilityNamedIam,
			},
		})
		if err != nil {
			log.Printf("[debug] [stack: %s] Error returned from CreateStack: %v\n", stackName, err)
			log.Fatalf("[error] [stack: %s] failed to create stack: %v", stackName, err)
		}
		log.Printf("[info] [stack: %s] Stack creation initiated. Check AWS CloudFormation console for progress.", stackName)
	}
	// wait until deployment succeeds for fails
	for {
		status, err := stackStatus(ctx, cf, stackName)
		if err != nil {
			return err
		}
		if strings.HasSuffix(status, "FAILED") {
			log.Fatalf("[error] [stack: %s] stack deployment failed with status %s", stackName, status)
		} else if strings.HasSuffix(status, "TERMINATED") {
			log.Fatalf("[error] [stack: %s] stack deployment terminated with status %s", stackName, status)
		} else if strings.HasSuffix(status, "ROLLBACK_COMPLETE") {
			log.Fatalf("[error] [stack: %s] stack deployment rolled back with status %s", stackName, status)
		} else if strings.HasSuffix(status, "COMPLETE") {
			log.Printf("[info] [stack: %s] Stack deployment completed with status %s", stackName, status)
			return nil
		}
		// TODO: show deployment events
		time.Sleep(5 * time.Second)
	}
}

// Helper to check substring ignoring case
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
