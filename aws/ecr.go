package aws

import (
	"context"
	"encoding/base64"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

type EcrAuthInfo struct {
	Username        string
	Password        string
	RegistryAddress string
}

func EcrAuthenticate(ctx context.Context) (*EcrAuthInfo, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("failed to load AWS configuration: %v", err)
		return nil, err
	}

	// Create ecr client
	ecrClient := ecr.NewFromConfig(cfg)
	// Get authorization token for your registry
	// You can specify registry IDs if you have multiple, otherwise it uses the default
	authOutput, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		log.Fatalf("failed to get ECR authorization token: %v", err)
		return nil, err
	}

	if len(authOutput.AuthorizationData) == 0 {
		log.Fatal("no authorization data received from ECR")
		return nil, err
	}

	// The authorization token is base64 encoded "AWS:<password>"
	authToken := authOutput.AuthorizationData[0].AuthorizationToken
	proxyEndpoint := authOutput.AuthorizationData[0].ProxyEndpoint // This is your ECR registry URI

	decodedToken, err := base64.StdEncoding.DecodeString(*authToken)
	if err != nil {
		log.Fatalf("failed to decode authorization token: %v", err)
		return nil, err
	}

	parts := strings.SplitN(string(decodedToken), ":", 2)
	if len(parts) != 2 {
		log.Fatal("invalid authorization token format")
	}

	username := parts[0]
	password := parts[1]
	registryAddress := strings.TrimPrefix(*proxyEndpoint, "https://") // Docker AuthConfig expects hostname

	return &EcrAuthInfo{
		Username:        username,
		Password:        password,
		RegistryAddress: registryAddress,
	}, nil
}
