package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/valendra-tech/aws-agent-trust-advisor/internal/services/logger"
)

// Service provides AWS functionality
type Service struct {
	cfg    aws.Config
	logger *logger.Logger
}

// Config holds the configuration for the AWS service
type Config struct {
	Profile string
	Region  string
}

// New creates a new AWS service instance
func New(ctx context.Context, cfg Config, log *logger.Logger) (*Service, error) {
	log.Info("Initializing AWS service with profile: %s", cfg.Profile)

	var opts []func(*config.LoadOptions) error

	if cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.Profile))
	}

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Error("Failed to load AWS configuration: %v", err)
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	return &Service{
		cfg:    awsCfg,
		logger: log,
	}, nil
}

// GetCallerIdentity returns the AWS account and user information
func (s *Service) GetCallerIdentity(ctx context.Context) (*sts.GetCallerIdentityOutput, error) {
	s.logger.Debug("Getting AWS caller identity")

	stsClient := sts.NewFromConfig(s.cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		s.logger.Error("Failed to get caller identity: %v", err)
		return nil, fmt.Errorf("failed to get caller identity: %w", err)
	}

	s.logger.Info("Successfully authenticated as AWS account: %s", *identity.Account)
	return identity, nil
}

// GetConfig returns the underlying AWS config
func (s *Service) GetConfig() aws.Config {
	return s.cfg
}
