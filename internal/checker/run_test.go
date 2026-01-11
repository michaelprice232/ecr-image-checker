package checker

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
)

func Test_validate(t *testing.T) {
	awsAccountID := "111111111111"
	awsRegion := "eu-west-1"
	repoName := "repo-1"
	tagName := "alpine"
	targetPlatforms := []string{"linux/arm64", "linux/amd64"}

	cases := []struct {
		testName    string
		keyName     string
		conf        repoConfig
		expectError bool
	}{
		{
			testName: "Happy path",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
						AwsRegion:    aws.String(awsRegion),
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Repo name unset",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
						AwsRegion:    aws.String(awsRegion),
					},
				},
			},
			expectError: true,
		},
		{
			testName: "Repo tag unset",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				TargetPlatforms:     targetPlatforms,
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
						AwsRegion:    aws.String(awsRegion),
					},
				},
			},
			expectError: true,
		},
		{
			testName: "No targets set",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
			},
			expectError: true,
		},
		{
			testName: "No target platforms set",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
						AwsRegion:    aws.String(awsRegion),
					},
				},
			},
			expectError: true,
		},
		{
			testName: "Empty target platform target",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     []string{"linux/amd64", ""},
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
						AwsRegion:    aws.String(awsRegion),
					},
				},
			},
			expectError: true,
		},
		{
			testName: "Use default AWS account ID",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				Targets: []*Target{
					{
						AwsRegion: aws.String(awsRegion),
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Use default AWS region",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
					},
				},
			},
			expectError: false,
		},
		{
			testName: "Region not set either explicitly or via default",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			c := config{repos: make(map[string]repoConfig)}

			c.repos[tc.keyName] = tc.conf

			err := c.validate()

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

}
