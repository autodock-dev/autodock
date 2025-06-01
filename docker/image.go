package docker

import (
	"autodock/aws"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/term"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	dockerImageTypes "github.com/docker/docker/api/types/image"
	dockerRegistryTypes "github.com/docker/docker/api/types/registry"
)

// Build docker image for a service
func BuildImage(ctx context.Context, service *composeTypes.ServiceConfig) string {
	ecrAuthInfo, err := aws.EcrAuthenticate(ctx)
	if err != nil {
		log.Fatalf("failed to authenticate with ECR: %v", err)
	}
	imageTag := fmt.Sprintf("%s/%s:%s", ecrAuthInfo.RegistryAddress, service.Image, time.Now().Format("200601021504"))
	log.Printf("[info] Building image for service %s with tag %s", service.Name, imageTag)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("[error] Error creating Docker client: %v", err)
	}
	defer dockerClient.Close()

	buildConfig := service.Build
	if buildConfig == nil {
		log.Fatalf("[error] No build configuration found for service %s", service.Name)
	}
	log.Printf("[debug] Build config: %v", buildConfig)
	// Create a tar archive of the build context
	// This function handles .dockerignore files automatically
	tar, err := archive.TarWithOptions(buildConfig.Context, &archive.TarOptions{})
	if err != nil {
		log.Fatalf("[error] Error creating tar archive: %v", err)
	}
	defer tar.Close()

	log.Printf("[debug] Building image for %s on platform %s", service.Name, service.Platform)
	buildOptions := types.ImageBuildOptions{
		Dockerfile: buildConfig.Dockerfile,
		Platform:   service.Platform,
		Tags:       []string{imageTag},
		Remove:     true, // Remove intermediate containers after a successful build
	}

	// Build the image
	buildResponse, err := dockerClient.ImageBuild(ctx, tar, buildOptions)
	if err != nil {
		log.Fatalf("[error] Error building image: %v", err)
	}
	defer buildResponse.Body.Close()

	// Use jsonmessage.Display to show the push progress and errors
	// This function handles the different types of JSON messages in the stream
	fd, isTerm := term.GetFdInfo(os.Stdout)
	err = jsonmessage.DisplayJSONMessagesStream(buildResponse.Body, os.Stdout, fd, isTerm, nil)
	if err != nil {
		// jsonmessage.Display returns an error if it encounters an error message in the stream
		if err == io.EOF {
			// EOF is expected when the stream ends successfully
			log.Println("Image build stream ended.")
		} else {
			log.Fatalf("Error during image build: %v", err) // Error from the stream
		}
	}
	log.Println("[info] Docker image built successfully!")
	return imageTag
}

// Push docker image to container registry. Return the pushed image with tag
func PushImage(ctx context.Context, service *composeTypes.ServiceConfig, imageTag string) {
	ecrAuthInfo, err := aws.EcrAuthenticate(ctx)
	if err != nil {
		log.Fatalf("failed to authenticate with ECR: %v", err)
	}

	// Create Docker AuthConfig
	authConfig := dockerRegistryTypes.AuthConfig{
		Username:      ecrAuthInfo.Username,
		Password:      ecrAuthInfo.Password,
		ServerAddress: ecrAuthInfo.RegistryAddress,
	}
	// Encode AuthConfig to Based64
	authConfigBytes, err := json.Marshal(authConfig)
	if err != nil {
		log.Fatalf("failed to marshal authConfig: %v", err)
	}
	authBase64 := base64.URLEncoding.EncodeToString(authConfigBytes)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("[error] Error creating Docker client: %v", err)
	}
	defer dockerClient.Close()

	pushOptions := dockerImageTypes.PushOptions{
		RegistryAuth: authBase64,
	}

	imageTags := []string{
		imageTag,
		fmt.Sprintf("%s/%s:latest", ecrAuthInfo.RegistryAddress, service.Image),
	}

	// tag the image with :latest
	if err := dockerClient.ImageTag(ctx, imageTag, fmt.Sprintf("%s/%s:latest", ecrAuthInfo.RegistryAddress, service.Image)); err != nil {
		log.Fatalf("failed to tag image: %v", err)
	}

	for _, tag := range imageTags {
		pushResponse, err := dockerClient.ImagePush(ctx, tag, pushOptions)
		if err != nil {
			log.Fatalf("failed to push image: %v", err)
		}
		defer pushResponse.Close()

		// Get terminal info for formatting the output
		fd, isTerm := term.GetFdInfo(os.Stdout)

		// Use jsonmessage.Display to show the push progress and errors
		// This function handles the different types of JSON messages in the stream
		err = jsonmessage.DisplayJSONMessagesStream(pushResponse, os.Stdout, fd, isTerm, nil)
		if err != nil {
			// jsonmessage.Display returns an error if it encounters an error message in the stream
			if err == io.EOF {
				// EOF is expected when the stream ends successfully
				log.Println("Image push stream ended.")
			} else {
				log.Fatalf("Error during image push: %v", err) // Error from the stream
			}
		}
	}
	log.Println("[info] Docker image pushed to ECR successfully!")
}
