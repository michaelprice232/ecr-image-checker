package checker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigFile = "config-defaults.yml"
	childConfigFile   = "config.yml"
	appName           = "ecr-image-checker"
)

type repoConfig struct {
	Region          *string  `yaml:"aws_region" json:"region"`
	AwsAccountId    *string  `yaml:"aws_account_id" json:"aws_account_id"`
	AwsRoleName     *string  `yaml:"aws_role_name" json:"aws_role_name"`
	RepoName        *string  `yaml:"repo_name" json:"repo_name"`
	RepoTag         *string  `yaml:"repo_tag" json:"repo_tag"`
	TargetPlatforms []string `yaml:"target_platforms" json:"target_platforms"`

	// Calculated fields not passed via YAML
	WorkingDirectory string `json:"working_directory"`
	FullImageRef     string `json:"full_image_ref"`
	RemoteTagMissing bool   `json:"remote_tag_missing"`
	AWSRoleARN       string `json:"aws_role_arn"`
}

type config struct {
	repos map[string]repoConfig

	// AWS clients
	stsClient *sts.Client
	ecrClient *ecr.Client
}

func newConfig() (config, error) {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return config{}, fmt.Errorf("loading AWS config: %w", err)
	}

	// ECR client is initialized dynamically for each target account/region combo
	stsClient := sts.NewFromConfig(awsCfg)

	c := config{
		repos:     make(map[string]repoConfig),
		stsClient: stsClient,
	}

	return c, nil
}

func Run(imageDirectory string) error {
	slog.Info("Base image directory", "path", imageDirectory)

	c, err := newConfig()
	if err != nil {
		return fmt.Errorf("creating new config: %w", err)
	}

	// Parse default config file
	defaultConfigData, err := parseYAMLFile(defaultConfigFile)
	if err != nil {
		return fmt.Errorf("parsing default YAML file (%s): %w", defaultConfigFile, err)
	}

	// Parse individual image directories
	if err = c.parseChildConfig(imageDirectory, defaultConfigData); err != nil {
		return fmt.Errorf("parsing child YAML files under %s: %w", imageDirectory, err)
	}

	c.addCalculatedFields()

	if err = c.checkECRImageTags(); err != nil {
		return fmt.Errorf("checking ECR tags: %w", err)
	}

	//c.displayConfig()

	if err = c.outputGitHubJSON(); err != nil {
		return fmt.Errorf("outputting GitHub JSON: %w", err)
	}

	return nil
}

func (c *config) parseChildConfig(imageDirectory string, defaultConfigData repoConfig) error {
	var sourceConfigFilePath string
	var finalConfigData repoConfig

	baseDirectories, err := os.ReadDir(imageDirectory)
	if err != nil {
		return fmt.Errorf("reading directories in %s: %w", imageDirectory, err)
	}

	for _, baseDir := range baseDirectories {
		// Ignore plain files and hidden directories
		if !baseDir.IsDir() || strings.HasPrefix(baseDir.Name(), ".") {
			continue
		}

		sourceConfigFilePath = path.Join(imageDirectory, baseDir.Name(), childConfigFile)

		childConfigData, err := parseYAMLFile(sourceConfigFilePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				slog.Warn("Skipping directory as child config file doesn't exist", "path", sourceConfigFilePath)
				continue
			} else {
				return fmt.Errorf("parsing YAML file (%s): %w", sourceConfigFilePath, err)
			}
		}
		slog.Info("Found child config file", "path", sourceConfigFilePath)

		// Merge the child config over the default config to determine the final config for this image
		finalConfigData = mergeRepoConfig(defaultConfigData, childConfigData)

		c.repos[sourceConfigFilePath] = finalConfigData
	}

	return nil
}

func (c *config) addCalculatedFields() {
	for key, repo := range c.repos {
		repo.FullImageRef = fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", *repo.AwsAccountId, *repo.Region, *repo.RepoName, *repo.RepoTag)
		repo.WorkingDirectory = path.Dir(key)

		if repo.AwsRoleName != nil && len(*repo.AwsRoleName) > 0 {
			repo.AWSRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", *repo.AwsAccountId, *repo.AwsRoleName)
		}

		c.repos[key] = repo
	}
}

func (c *config) checkECRImageTags() error {
	for key, repo := range c.repos {
		if err := c.setupECRClient(repo); err != nil {
			return fmt.Errorf("setting up ECR client: %w", err)
		}

		remoteTagMissing := true
		ecrImages := &ecr.ListImagesOutput{}
		var err error
		nextToken := ""

		for {
			listImagesInput := &ecr.ListImagesInput{
				RepositoryName: repo.RepoName,
				Filter:         &ecrTypes.ListImagesFilter{TagStatus: ecrTypes.TagStatusTagged},
			}

			if nextToken != "" {
				listImagesInput.NextToken = aws.String(nextToken)
			}

			ecrImages, err = c.ecrClient.ListImages(context.Background(), listImagesInput)
			if err != nil {
				return fmt.Errorf("listing Docker tags for %s: %w", *repo.RepoName, err)
			}

			if ecrImages != nil {
				for _, image := range ecrImages.ImageIds {
					if image.ImageTag != nil && *image.ImageTag == *repo.RepoTag {
						slog.Debug("Found image tag", "repo", *repo.RepoName, "tag", *repo.RepoTag)
						remoteTagMissing = false
						break
					}
				}
			}

			// Found remote image ref
			if !remoteTagMissing {
				break
			}

			// No more remote images to check
			if ecrImages == nil || ecrImages.NextToken == nil {
				break
			}

			nextToken = *ecrImages.NextToken
		}

		// Flag the Docker tag as needing to be built
		if remoteTagMissing {
			repo.RemoteTagMissing = true
			c.repos[key] = repo
		}
	}

	return nil
}

func (c *config) setupECRClient(repo repoConfig) error {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(), func(o *awsConfig.LoadOptions) error {
		o.Region = *repo.Region
		return nil
	})
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	if repo.AwsRoleName != nil {
		slog.Debug("Assuming role", "role", repo.AWSRoleARN, "repo", *repo.RepoName)
		creds := stscreds.NewAssumeRoleProvider(c.stsClient, repo.AWSRoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = appName
		})
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
		c.ecrClient = ecr.NewFromConfig(awsCfg)

		return nil
	}

	slog.Debug("No assume IAM role defined. Using normal credential chain", "repo", *repo.RepoName)
	c.ecrClient = ecr.NewFromConfig(awsCfg)

	return nil
}

func (c *config) displayConfig() {
	for key, childRepoConf := range c.repos {
		fmt.Printf("> Displaying %s\n", key)

		if childRepoConf.Region != nil {
			fmt.Printf("Region: %s\n", *childRepoConf.Region)
		}

		if childRepoConf.AwsAccountId != nil {
			fmt.Printf("AWS Account ID: %s\n", *childRepoConf.AwsAccountId)
		}

		if childRepoConf.RepoName != nil {
			fmt.Printf("Repo name: %s\n", *childRepoConf.RepoName)
		}

		if childRepoConf.RepoTag != nil {
			fmt.Printf("Repo tag: %s\n", *childRepoConf.RepoTag)
		}

		if childRepoConf.TargetPlatforms != nil && len(childRepoConf.TargetPlatforms) > 0 {
			fmt.Printf("Target platforms: %s\n", childRepoConf.TargetPlatforms)
		}

		if childRepoConf.AwsRoleName != nil {
			fmt.Printf("AWS role name: %s\n", *childRepoConf.AwsRoleName)
		}

		if childRepoConf.WorkingDirectory != "" {
			fmt.Printf("Working directory: %s\n", childRepoConf.WorkingDirectory)
		}

		if childRepoConf.FullImageRef != "" {
			fmt.Printf("Full image ref: %s\n", childRepoConf.FullImageRef)
		}

		if childRepoConf.AWSRoleARN != "" {
			fmt.Printf("AWS Role ARN: %s\n", childRepoConf.AWSRoleARN)
		}

		fmt.Printf("Remote tag missing: %t\n", childRepoConf.RemoteTagMissing)
	}
}

func (c *config) outputGitHubJSON() error {
	missingTags := filterMissingTags(c.repos)

	if len(missingTags) == 0 {
		return nil
	}

	b, err := json.Marshal(missingTags)
	if err != nil {
		return fmt.Errorf("marshalling JSON: %w", err)
	}

	fmt.Printf("targets=%s\n", string(b))

	return nil
}

func filterMissingTags(original map[string]repoConfig) []repoConfig {
	missingTags := make([]repoConfig, 0)

	for _, repo := range original {
		if repo.RemoteTagMissing {
			missingTags = append(missingTags, repo)
		}
	}

	return missingTags
}

func parseYAMLFile(path string) (repoConfig, error) {
	configData := repoConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		return configData, fmt.Errorf("opening YAML file (%s): %w", path, err)
	}

	if err = yaml.Unmarshal(data, &configData); err != nil {
		return configData, fmt.Errorf("parsing YAML file (%s): %w", path, err)
	}

	return configData, nil
}

func mergeRepoConfig(defaultConf, childRepoConf repoConfig) repoConfig {
	finalConf := defaultConf

	if childRepoConf.Region != nil {
		finalConf.Region = childRepoConf.Region
	}

	if childRepoConf.AwsAccountId != nil {
		finalConf.AwsAccountId = childRepoConf.AwsAccountId
	}

	if childRepoConf.AwsRoleName != nil {
		finalConf.AwsRoleName = childRepoConf.AwsRoleName
	}

	if childRepoConf.RepoName != nil {
		finalConf.RepoName = childRepoConf.RepoName
	}

	if childRepoConf.RepoTag != nil {
		finalConf.RepoTag = childRepoConf.RepoTag
	}

	if childRepoConf.TargetPlatforms != nil && len(childRepoConf.TargetPlatforms) > 0 {
		finalConf.TargetPlatforms = childRepoConf.TargetPlatforms
	}

	return finalConf
}
