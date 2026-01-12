package checker

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/stretchr/testify/require"
)

func Test_validate(t *testing.T) {
	awsAccountID := "111111111111"
	awsRegion := "eu-west-1"
	repoName := "repo-1"
	tagName := "alpine"
	targetPlatforms := []string{"linux/arm64", "linux/amd64"}
	buildArgs := map[string]string{"key": "value"}

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
				BuildArgs:           buildArgs,
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
		{
			testName: "AWS account ID not set either explicitly or via default",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultRegion:   aws.String(awsRegion),
				RepoName:        aws.String(repoName),
				RepoTag:         aws.String(tagName),
				TargetPlatforms: targetPlatforms,
				Targets: []*Target{
					{
						AwsRegion: aws.String(awsRegion),
					},
				},
			},
			expectError: true,
		},
		{
			testName: "Empty build_args map",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				BuildArgs:           map[string]string{}, // Empty
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
			testName: "Empty build_args value",
			keyName:  "image-1/config.yml",
			conf: repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				RepoName:            aws.String(repoName),
				RepoTag:             aws.String(tagName),
				TargetPlatforms:     targetPlatforms,
				BuildArgs: map[string]string{
					"key": "", // empty value
				},
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
						AwsRegion:    aws.String(awsRegion),
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

func Test_addCalculatedFields(t *testing.T) {
	t.Parallel()

	awsAccountID := "111111111111"
	awsRegion := "eu-west-1"
	iamRole := "my-iam-role"
	repoName := "repo-1"
	tagName := "alpine"
	buildArgs := map[string]string{
		"key":  "value",
		"key2": "value2",
	}
	targetPlatforms := []string{"linux/arm64", "linux/amd64"}
	pathOne := "path-one/config.yml"

	conf := map[string]repoConfig{
		pathOne: {
			RepoName:        aws.String(repoName),
			RepoTag:         aws.String(tagName),
			TargetPlatforms: targetPlatforms,
			BuildArgs:       buildArgs,
			Targets: []*Target{
				{
					AwsAccountId: aws.String(awsAccountID),
					AwsRegion:    aws.String(awsRegion),
					AwsRoleName:  aws.String(iamRole),
				},
			},
		},
	}

	c := config{repos: conf}
	c.addCalculatedFields()

	p1 := c.repos[pathOne]
	_, err := arn.Parse(p1.Targets[0].AWSRoleARN)
	require.NoError(t, err, "The computed aws_role_arn field must be a valid AWS ARN")

	require.NotEmpty(t, p1.Targets[0].FullImageRef)
	require.NotEmpty(t, p1.Targets[0].WorkingDirectory)

	if len(p1.TargetPlatforms) > 0 {
		require.NotEmpty(t, p1.Targets[0].TargetPlatformStr)
	}

	if len(p1.BuildArgs) > 0 {
		require.NotEmpty(t, p1.Targets[0].BuildArgsStr)
	}
}

func Test_outputGitHubJSON(t *testing.T) {
	t.Run("No targets need building", func(t *testing.T) {
		targets := make([]Target, 0)
		result, err := outputGitHubJSON(targets)
		require.NoError(t, err)
		require.Equal(t, "targets=[]", result)
	})

	t.Run("Valid JSON", func(t *testing.T) {
		targets := []Target{
			{
				FullImageRef:     "11111111111.dkr.ecr.eu-west-1.amazonaws.com/image-3:3",
				RemoteTagMissing: true,
			},
			{
				FullImageRef:     "22222222222.dkr.ecr.eu-west-2.amazonaws.com/image-3:3",
				RemoteTagMissing: true,
			},
		}
		result, err := outputGitHubJSON(targets)
		require.NoError(t, err)

		resultBreakdown := strings.Split(result, "=")
		require.Equal(t, 2, len(resultBreakdown))
		require.Equal(t, "targets", resultBreakdown[0])

		var unmarshalledTargets []Target
		err = json.Unmarshal([]byte(resultBreakdown[1]), &unmarshalledTargets)
		require.NoError(t, err)
		require.Equal(t, targets[1].FullImageRef, unmarshalledTargets[1].FullImageRef)
		require.Equal(t, len(targets), len(unmarshalledTargets))
	})
}

func Test_filterMissingTags(t *testing.T) {
	cases := []struct {
		testName       string
		conf           map[string]repoConfig
		expectedLength int
	}{
		{
			testName: "1 config file 1 match",
			conf: map[string]repoConfig{
				"image-2/config.yml": {
					RepoName: aws.String("image-2"),
					Targets:  []*Target{{RemoteTagMissing: true}},
				},
			},
			expectedLength: 1,
		},
		{
			testName:       "nil input",
			conf:           nil,
			expectedLength: 0,
		},
		{
			testName: "2 config file multiple match",
			conf: map[string]repoConfig{
				"image-2/config.yml": {
					RepoName: aws.String("image-2"),
					Targets: []*Target{
						{
							FullImageRef:     "11111111111.dkr.ecr.eu-west-1.amazonaws.com/image-3:3",
							RemoteTagMissing: true,
						},
					},
				},
				"image-3/config.yml": {
					RepoName: aws.String("image-3"),
					Targets: []*Target{
						{
							FullImageRef:     "11111111111.dkr.ecr.eu-west-1.amazonaws.com/image-3:3",
							RemoteTagMissing: true,
						},
						{
							FullImageRef:     "222222222222.dkr.ecr.eu-west-1.amazonaws.com/image-3:3",
							RemoteTagMissing: false,
						},
					},
				},
			},
			expectedLength: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			result := filterMissingTags(tc.conf)

			require.Equal(t, tc.expectedLength, len(result))
		})
	}
}

func Test_strPtrEmpty(t *testing.T) {
	s := "non-empty-string"
	result := strPtrEmpty(&s)
	require.False(t, result)

	s = ""
	result = strPtrEmpty(&s)
	require.True(t, result)

	result = strPtrEmpty(nil)
	require.True(t, result)
}

func Test_readStrPointer(t *testing.T) {
	s := "non-empty-string"
	result := readStrPointer(&s)
	require.Equal(t, s, result)

	s = ""
	result = readStrPointer(&s)
	require.Zero(t, s)

	result = readStrPointer(nil)
	require.Zero(t, s)
}

func Test_parseYAMLFile(t *testing.T) {
	result, err := parseYAMLFile("testdata/config-child.yml")
	require.NoError(t, err)
	require.NotNil(t, result.RepoName)
	require.NotNil(t, result.RepoTag)
	require.Len(t, result.TargetPlatforms, 2)
	require.Len(t, result.Targets, 2)

	require.NotNil(t, result.Targets[0].AwsAccountId)
	require.Equal(t, "077439031059", *result.Targets[0].AwsAccountId)

	require.NotNil(t, result.Targets[1].AwsRegion)
	require.Equal(t, "ap-northeast-1", *result.Targets[1].AwsRegion)

	_, err = parseYAMLFile("invalid-path.yml")
	require.Error(t, err)
}

func Test_mergeRepoConfig(t *testing.T) {
	awsAccountID := "111111111111"
	awsRegion := "eu-west-1"
	awsRole := "my-iam-role"
	repoName := "repo-1"
	tagName := "alpine"
	targetPlatforms := []string{"linux/arm64", "linux/amd64"}
	buildArgs := map[string]string{"key": "value"}

	cases := []struct {
		testName    string
		defaultConf *repoConfig
		childConf   *repoConfig
	}{
		{
			testName: "Use default AWS account ID",
			defaultConf: &repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				DefaultAwsRoleName:  aws.String(awsRole),
			},
			childConf: &repoConfig{
				RepoName:        aws.String(repoName),
				RepoTag:         aws.String(tagName),
				TargetPlatforms: targetPlatforms,
				BuildArgs:       buildArgs,
				Targets: []*Target{
					{
						AwsRegion:   aws.String(awsRegion),
						AwsRoleName: aws.String(awsRole),
					},
				},
			},
		},
		{
			testName: "Use default AWS region and IAM role",
			defaultConf: &repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				DefaultAwsRoleName:  aws.String(awsRole),
			},
			childConf: &repoConfig{
				RepoName:        aws.String(repoName),
				RepoTag:         aws.String(tagName),
				TargetPlatforms: targetPlatforms,
				BuildArgs:       buildArgs,
				Targets: []*Target{
					{
						AwsAccountId: aws.String(awsAccountID),
					},
				},
			},
		},
		{
			testName: "Missing targets key use defaults",
			defaultConf: &repoConfig{
				DefaultAwsAccountId: aws.String(awsAccountID),
				DefaultRegion:       aws.String(awsRegion),
				DefaultAwsRoleName:  aws.String(awsRole),
			},
			childConf: &repoConfig{
				RepoName:        aws.String(repoName),
				RepoTag:         aws.String(tagName),
				TargetPlatforms: targetPlatforms,
				BuildArgs:       buildArgs,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			result := mergeRepoConfig(tc.defaultConf, tc.childConf)

			require.NotNil(t, result.Targets[0].AwsAccountId)
			require.Equal(t, awsAccountID, *result.Targets[0].AwsAccountId)

			require.NotNil(t, result.Targets[0].AwsRegion)
			require.Equal(t, awsRegion, *result.Targets[0].AwsRegion)

			if tc.defaultConf.DefaultAwsRoleName != nil || tc.childConf.Targets[0].AwsRoleName != nil {
				require.Equal(t, awsRole, *tc.childConf.Targets[0].AwsRoleName)
			}

			if tc.childConf.Targets == nil || len(tc.childConf.Targets) == 0 {
				if tc.defaultConf.DefaultAwsAccountId != nil && tc.defaultConf.DefaultRegion != nil {
					require.Equal(t, awsAccountID, *result.Targets[0].AwsAccountId)
					require.Equal(t, awsRegion, *result.Targets[0].AwsRegion)
				}
			}
		})
	}

}

func Test_parseChildConfig(t *testing.T) {
	imageDir := "testdata/image-dir"
	childConfigOneKey := fmt.Sprintf("%s/image-1/%s", imageDir, childConfigFile)
	childConfigTwoKey := fmt.Sprintf("%s/image-2/%s", imageDir, childConfigFile)
	c := config{repos: make(map[string]repoConfig)}
	fullDefaultData := repoConfig{
		DefaultAwsAccountId: aws.String("111111111111"),
		DefaultRegion:       aws.String("eu-west-3"),
		DefaultAwsRoleName:  aws.String("my-iam-role"),
	}

	err := c.parseChildConfig(imageDir, fullDefaultData)
	require.NoError(t, err)
	require.Len(t, c.repos, 2)
	require.Len(t, c.repos[childConfigOneKey].Targets, 2)
	require.Len(t, c.repos[childConfigTwoKey].Targets, 1, "targets should be auto populated from defaults")
}
