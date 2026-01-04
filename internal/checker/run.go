package checker

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"

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
}

type config struct {
	repos map[string]repoConfig
}

func newConfig() (config, error) {
	c := config{
		repos: make(map[string]repoConfig),
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

	c.displayConfig()

	return nil
}

func (c *config) addCalculatedFields() {
	for idx, repo := range c.repos {
		repo.FullImageRef = fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:%s", *repo.AwsAccountId, *repo.Region, *repo.RepoName, *repo.RepoTag)
		repo.WorkingDirectory = path.Dir(idx)

		c.repos[idx] = repo
	}
}

func (c *config) parseChildConfig(imageDirectory string, defaultConfigData repoConfig) error {
	var sourceConfigFilePath string
	var finalConfigData repoConfig

	baseDirectories, err := os.ReadDir(imageDirectory)
	if err != nil {
		return fmt.Errorf("reading directories in %s: %w", imageDirectory, err)
	}

	for _, baseDir := range baseDirectories {
		// Ignore hidden directories and plain files
		if !baseDir.IsDir() || strings.HasPrefix(baseDir.Name(), ".") {
			continue
		}

		sourceConfigFilePath = path.Join(imageDirectory, baseDir.Name(), childConfigFile)

		childConfigData, err := parseYAMLFile(sourceConfigFilePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				slog.Warn("Skipping child config file as it doesn't exist", "path", sourceConfigFilePath)
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

func (c *config) displayConfig() {
	for idx, repoConf := range c.repos {
		fmt.Printf("> Displaying %s\n", idx)

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

func mergeRepoConfig(defaultConf, repoConf repoConfig) repoConfig {
	finalConf := defaultConf

	if repoConf.Region != nil {
		finalConf.Region = repoConf.Region
	}

	if repoConf.AwsAccountId != nil {
		finalConf.AwsAccountId = repoConf.AwsAccountId
	}

	if repoConf.RepoName != nil {
		finalConf.RepoName = repoConf.RepoName
	}

	if repoConf.RepoTag != nil {
		finalConf.RepoTag = repoConf.RepoTag
	}

	if repoConf.TargetPlatforms != nil && len(repoConf.TargetPlatforms) > 0 {
		finalConf.TargetPlatforms = repoConf.TargetPlatforms
	}

	if repoConf.AwsRoleName != nil {
		finalConf.AwsRoleName = repoConf.AwsRoleName
	}

	return finalConf
}
