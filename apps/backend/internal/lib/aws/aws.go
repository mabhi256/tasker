package aws

import (
	"context"

	aws "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/mabhi256/tasker/internal/server"
)

type AWS struct {
	S3 *S3Client
}

func NewAWS(server *server.Server) (*AWS, error) {
	awsConfig := server.Config.AWS

	configOptions := []func(*aws.LoadOptions) error{
		aws.WithRegion(awsConfig.Region),
		aws.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			awsConfig.AccessKeyID,
			awsConfig.SecretAccessKey,
			"",
		)),
	}

	// Add custom endpoint if provided (for S3-compatible services like Sevalla)
	if awsConfig.EndpointURL != "" {
		configOptions = append(configOptions, aws.WithBaseEndpoint(awsConfig.EndpointURL))
	}

	cfg, err := aws.LoadDefaultConfig(context.TODO(), configOptions...)
	if err != nil {
		return nil, err
	}

	return &AWS{
		S3: NewS3Client(server, cfg),
	}, nil
}
