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

func Lint() error {
	slog.Info("Linting...")

	return nil
}

type repoConfig struct {
	Region          *string  `yaml:"aws_region"`
	AwsAccountId    *string  `yaml:"aws_account_id"`
	RepoName        *string  `yaml:"repo_name"`
	RepoTag         *string  `yaml:"repo_tag"`
	TargetPlatforms []string `yaml:"target_platforms"`
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

	_, err := newConfig()
	if err != nil {
		return fmt.Errorf("creating new config: %w", err)
	}

	// Parse default config file
	defaultData, err := os.ReadFile(defaultConfigFile)
	if err != nil {
		return fmt.Errorf("opening default config file (%s): %w", defaultConfigFile, err)
	}

	defaultConfigData := repoConfig{}
	if err := yaml.Unmarshal(defaultData, &defaultConfigData); err != nil {
		return fmt.Errorf("parsing YAML in default config file (%s): %w", defaultConfigFile, err)
	}

	fmt.Println("> Default config struct:")
	displayRepoConfig(defaultConfigData)

	// Parse individual image directories
	baseDirectories, err := os.ReadDir(imageDirectory)
	if err != nil {
		return fmt.Errorf("reading directories in %s: %w", imageDirectory, err)
	}

	for _, baseDir := range baseDirectories {
		if !baseDir.IsDir() || strings.HasPrefix(baseDir.Name(), ".") {
			continue
		}

		sourceConfigFilePath := path.Join(imageDirectory, baseDir.Name(), childConfigFile)

		childData, err := os.ReadFile(sourceConfigFilePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				slog.Warn("Skipping child config file as it doesn't exist", "path", sourceConfigFilePath)
				continue
			} else {
				return fmt.Errorf("opening child config file (%s): %w", sourceConfigFilePath, err)
			}
		}

		slog.Info("Found child config file", "path", sourceConfigFilePath)

		childConfigData := repoConfig{}
		if err = yaml.Unmarshal(childData, &childConfigData); err != nil {
			return fmt.Errorf("parsing YAML in child config file (%s): %w", sourceConfigFilePath, err)
		}

		fmt.Println("> Child config struct:")
		displayRepoConfig(childConfigData)

		// Merge the child config over the default config to determine the final config
		finalConfigData := mergeRepoConfig(defaultConfigData, childConfigData)
		fmt.Println("> Final config struct:")
		displayRepoConfig(finalConfigData)
	}

	return nil
}

func displayRepoConfig(repoConf repoConfig) {
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

	return finalConf
}
