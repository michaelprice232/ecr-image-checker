# ECR Image Checker

`ecr-image-checker` is a small Go utility designed to be used inside **GitHub Actions workflows** to manage **multiple AWS ECR Docker images stored in a single GitHub repository**.

It solves a very common problem:

You have several small utility images or lightly-modified upstream images that don’t justify separate repositories, but you still want clean workflows, parallel builds, and immutable ECR safety.

This tool parses per-image configuration, checks whether image tags already exist in ECR, and outputs a GitHub Actions–compatible matrix containing only the images that actually need building.

## Key Features

- Supports multiple Docker images in one repo
- Supports building for multiple AWS accounts and regions
- Avoids duplicate builds and immutable ECR failures
- Outputs JSON compatible with GitHub Actions matrix builds
- Supports OIDC-only or OIDC + cross-account assume-role
- Builds images in parallel
- Keeps workflows simple and readable
- Written in Go, distributed as a single binary

## Repository Structure

Each Docker image must live in its own directory and contain:

- Dockerfile
- config.yml

Example:

```text
.
├── config-defaults.yml
├── image-a/
│   ├── Dockerfile
│   └── config.yml
└── image-b/
    ├── Dockerfile
    └── config.yml
```

By default, the project root is scanned.
You can override this using the `IMAGE_DIRECTORY` environment variable to set a new base directory.

## Configuration

### Root Defaults (config-defaults.yml)

These will be inherited by each image’s `config.yml` unless overridden.

```yaml
default_aws_account_id: 11111111111
default_aws_region: eu-west-1
default_aws_role_name: mike-ecr-query
```

### Image Config (config.yml)

```yaml
repo_name: mike-test
repo_tag: alpine-3
target_platforms:
  - "linux/arm64"
  - "linux/amd64"

build_args:
  BASE_IMAGE_TAG: "3"

# If NO targets key is specified, use the defaults. Useful for single account/region deployment
# If PART of the targets are missing, complete using the defaults
targets:
  # Inherits defaults
  - aws_account_id: 11111111111

#  # Explicit
  - aws_account_id: 22222222222
    aws_region: ap-northeast-1
    aws_role_name: mike-ecr-query # assumes an IAM role when checking the ECR Docker tags
```

## How It Works

1. Scan for config.yml files
2. Merge with config-defaults.yml
3. Check ECR for existing tags
4. Skip existing images
5. Output GitHub Actions matrix JSON
6. A separate GitHub job in the workflow builds the images using the standard tooling

## Environment Variables

`IMAGE_DIRECTORY` – base directory to scan for image config

`LOG_LEVEL` – debug, info, warn, error

## IAM Roles

The app is typically run in a GitHub workflow using an OIDC federated IAM role to grant AWS permissions.
Depending on your setup there are two options for connecting to the various AWS accounts to build images.

### Using IAM Assume Roles

The app will assume IAM roles when connecting to each account target to check for image tags.
The target matrix build job will use IAM role chaining via the normal `configure-aws-credentials` action.
The IAM role must have the appropriate trust policy.

An example IAM policy is [here](./examples/aws-policies/cross-account-assume-role).

The `aws_role_name` must be set in either the child `config.yml` file or via a default (`default_aws_role_name`).

### Using the base IAM Role

The app will just use the base permissions assigned to the OIDC federated role.
If going cross AWS account then you need to ensure the ECR repo resource policies are set up for this.
There is an example [here](./examples/aws-policies/ecr-cross-account).

## Example Workflows

1. [Using a base OIDC role and cross account assume roles](./examples/gh-workflows/with-assume-roles/workflow.yml)
2. [Using just the base OIDC role](./examples/gh-workflows/without-roles/workflow.yml)

## Running Locally

```shell
# Units tests
make test

# Coverage HTML report
make coverage-html

# Run the app using your config
AWS_PROFILE=<profile> LOG_LEVEL=debug IMAGE_DIRECTORY=<dir> make run
```