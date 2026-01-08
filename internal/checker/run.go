package checker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigFile = "config-defaults.yml"
	childConfigFile   = "config.yml"
)

type repoConfig struct {
	Region          *string  `yaml:"aws_region"`
	AwsAccountId    *string  `yaml:"aws_account_id"`
	AwsRoleName     *string  `yaml:"aws_role_name"`
	RepoName        *string  `yaml:"repo_name"`
	RepoTag         *string  `yaml:"repo_tag"`
	TargetPlatforms []string `yaml:"target_platforms"`

	// Calculated fields not passed via YAML
	WorkingDirectory string
	FullImageRef     string
	RemoteTagMissing bool
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
	stsClient := sts.NewFromConfig(awsCfg)
	ecrClient := ecr.NewFromConfig(awsCfg)

	c := config{
		repos:     make(map[string]repoConfig),
		stsClient: stsClient,
		ecrClient: ecrClient,
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

	c.displayConfig()

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

		c.repos[key] = repo
	}
}

func (c *config) checkECRImageTags() error {
	// todo: assume role in target account and region

	for key, repo := range c.repos {
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
						slog.Info("Found image tag", "repo", *repo.RepoName, "tag", *repo.RepoTag)
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

func (c *config) displayConfig() {
	for key, repoConf := range c.repos {
		fmt.Printf("> Displaying %s\n", key)

		if repoConf.Region != nil {
			fmt.Printf("Region: %s\n", *repoConf.Region)
		}

		if repoConf.AwsAccountId != nil {
			fmt.Printf("AWS Account ID: %s\n", *repoConf.AwsAccountId)
		}

		if repoConf.RepoName != nil {
			fmt.Printf("Repo name: %s\n", *repoConf.RepoName)
		}

		if repoConf.RepoTag != nil {
			fmt.Printf("Repo tag: %s\n", *repoConf.RepoTag)
		}

		if repoConf.TargetPlatforms != nil && len(repoConf.TargetPlatforms) > 0 {
			fmt.Printf("Target platforms: %s\n", repoConf.TargetPlatforms)
		}

		if repoConf.AwsRoleName != nil {
			fmt.Printf("AWS role name: %s\n", *repoConf.AwsRoleName)
		}

		if repoConf.WorkingDirectory != "" {
			fmt.Printf("Working directory: %s\n", repoConf.WorkingDirectory)
		}

		if repoConf.FullImageRef != "" {
			fmt.Printf("Full image ref: %s\n", repoConf.FullImageRef)
		}

		fmt.Printf("Remote tag missing: %t\n", repoConf.RemoteTagMissing)
	}
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
