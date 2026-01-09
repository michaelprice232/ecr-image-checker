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

type Target struct {
	AwsAccountId *string `yaml:"aws_account_id" json:"aws_account_id"`
	AwsRegion    *string `yaml:"aws_region" json:"aws_region"`
	AwsRoleName  *string `yaml:"aws_role_name" json:"aws_role_name"`

	// Calculated fields not passed via YAML
	AWSRoleARN        string `json:"aws_role_arn"`
	FullImageRef      string `json:"full_image_ref"`
	RemoteTagMissing  bool   `json:"remote_tag_missing"`
	WorkingDirectory  string `json:"working_directory"`
	TargetPlatformStr string `json:"target_platforms"`
}

type repoConfig struct {
	// Defaults, which can be overridden in the Targets
	DefaultAwsAccountId *string `yaml:"default_aws_account_id" json:"default_aws_account_id"`
	DefaultRegion       *string `yaml:"default_aws_region" json:"default_aws_region"`
	DefaultAwsRoleName  *string `yaml:"default_aws_role_name" json:"default_aws_role_name"`

	RepoName        *string   `yaml:"repo_name" json:"repo_name"`
	RepoTag         *string   `yaml:"repo_tag" json:"repo_tag"`
	TargetPlatforms []string  `yaml:"target_platforms" json:"target_platforms_slice"`
	Targets         []*Target `yaml:"targets" json:"targets"`
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

	output, err := c.outputGitHubJSON()
	if err != nil {
		return fmt.Errorf("outputting GitHub JSON: %w", err)
	}

	// Output JSON to stdout which can be consumed by GitHub workflow matrix via an output
	fmt.Println(output)

	return nil
}

func (c *config) parseChildConfig(imageDirectory string, defaultConfigData repoConfig) error {
	var sourceConfigFilePath string
	var finalConfigData *repoConfig

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
		if finalConfigData, err = mergeRepoConfig(&defaultConfigData, &childConfigData); err != nil {
			return fmt.Errorf("merging config: %w", err)
		}

		c.repos[sourceConfigFilePath] = *finalConfigData

		for _, target := range finalConfigData.Targets {
			slog.Debug("Child config",
				"path", sourceConfigFilePath,
				"aws_region", readStrPointer(target.AwsRegion),
				"aws_account_id", readStrPointer(target.AwsAccountId),
				"aws_role_name", readStrPointer(target.AwsRoleName),
				"repo_name", readStrPointer(finalConfigData.RepoName),
				"repo_tag", readStrPointer(finalConfigData.RepoTag),
			)
		}

	}

	return nil
}

func (c *config) addCalculatedFields() {
	for key, repo := range c.repos {
		for _, target := range repo.Targets {
			if target.AwsRoleName != nil && len(*target.AwsRoleName) > 0 {
				target.AWSRoleARN = fmt.Sprintf("arn:aws:iam::%s:role/%s", *target.AwsAccountId, *target.AwsRoleName)
			}

			target.FullImageRef = fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", *target.AwsAccountId, *target.AwsRegion, *repo.RepoName, *repo.RepoTag)

			target.WorkingDirectory = path.Dir(key)

			if repo.TargetPlatforms != nil && len(repo.TargetPlatforms) > 0 {
				target.TargetPlatformStr = strings.Join(repo.TargetPlatforms, ",")
			}
		}

		c.repos[key] = repo
	}
}

func (c *config) checkECRImageTags() error {
	for key, repo := range c.repos {
		for idx, target := range repo.Targets {
			if err := c.setupECRClient(*target, *repo.RepoName); err != nil {
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
				target.RemoteTagMissing = true
				c.repos[key].Targets[idx] = target
			}
		}
	}

	return nil
}

func (c *config) setupECRClient(target Target, repoName string) error {
	awsCfg, err := awsConfig.LoadDefaultConfig(context.Background(), func(o *awsConfig.LoadOptions) error {
		o.Region = *target.AwsRegion
		return nil
	})
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}

	if target.AwsRoleName != nil {
		slog.Debug("Assuming role", "role", target.AWSRoleARN, "repo", repoName)
		creds := stscreds.NewAssumeRoleProvider(c.stsClient, target.AWSRoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = appName
		})
		awsCfg.Credentials = aws.NewCredentialsCache(creds)
		c.ecrClient = ecr.NewFromConfig(awsCfg)

		return nil
	}

	slog.Debug("No assume IAM role defined. Using normal credential chain", "repo", repoName)
	c.ecrClient = ecr.NewFromConfig(awsCfg)

	return nil
}

func (c *config) outputGitHubJSON() (string, error) {
	missingTags := filterMissingTags(c.repos)

	// No Docker images to build
	if len(missingTags) == 0 {
		return "targets=[]", nil
	}

	b, err := json.Marshal(missingTags)
	if err != nil {
		return "", fmt.Errorf("marshalling JSON: %w", err)
	}

	return fmt.Sprintf("targets=%s\n", string(b)), nil
}

func filterMissingTags(original map[string]repoConfig) []Target {
	missingTags := make([]Target, 0)

	for _, repo := range original {
		for _, target := range repo.Targets {
			if target.RemoteTagMissing {
				missingTags = append(missingTags, *target)
			}
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

func mergeRepoConfig(defaultConf, childRepoConf *repoConfig) (*repoConfig, error) {
	for _, target := range childRepoConf.Targets {
		if target.AwsAccountId == nil {
			if defaultConf.DefaultAwsAccountId != nil && len(*defaultConf.DefaultAwsAccountId) > 0 {
				target.AwsAccountId = defaultConf.DefaultAwsAccountId
				slog.Debug("Using default config value", "repo", *childRepoConf.RepoName, "aws_account_id", *defaultConf.DefaultAwsAccountId)
			} else {
				return nil, fmt.Errorf("AWS account ID not set for both the child and default configs for repo: %s", *childRepoConf.RepoName)
			}
		}

		if target.AwsRegion == nil {
			if defaultConf.DefaultRegion != nil && len(*defaultConf.DefaultRegion) > 0 {
				target.AwsRegion = defaultConf.DefaultRegion
				slog.Debug("Using default config value", "repo", *childRepoConf.RepoName, "aws_region", *defaultConf.DefaultRegion)
			} else {
				return nil, fmt.Errorf("AWS region not set for both the child and default configs for repo: %s", *childRepoConf.RepoName)
			}
		}

		// Non-mandatory
		if target.AwsRoleName == nil {
			if defaultConf.DefaultAwsRoleName != nil && len(*defaultConf.DefaultAwsRoleName) > 0 {
				target.AwsRoleName = defaultConf.DefaultAwsRoleName
				slog.Debug("Using default config value", "repo", *childRepoConf.RepoName, "aws_role_name", *defaultConf.DefaultAwsRoleName)
			}
		}
	}

	// No targets key entirely. Fall back to defaults
	if childRepoConf.Targets == nil || (childRepoConf.Targets != nil && len(childRepoConf.Targets) == 0) {
		if defaultConf.DefaultAwsAccountId != nil && defaultConf.DefaultRegion != nil {
			childRepoConf.Targets = []*Target{
				{
					AwsAccountId: defaultConf.DefaultAwsAccountId,
					AwsRegion:    defaultConf.DefaultRegion,
				},
			}

			if defaultConf.DefaultAwsRoleName != nil {
				childRepoConf.Targets[0].AwsRoleName = defaultConf.DefaultAwsRoleName
			}

			slog.Debug("Using default config value", "repo", *childRepoConf.RepoName, "target", childRepoConf.Targets)
		}
	}

	return childRepoConf, nil
}

func readStrPointer(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}
