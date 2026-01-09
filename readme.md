# ecr-image-checker

An app which parses the local repo for Docker config - one per root level directory - and checks whether the Docker tags exist in AWS ECR.
It then outputs JSON config which is compatible with GitHub Action matrix builds for any that are missing so that they can be built.
Designed to help create clean GitHub Action workflows for building Docker images for ECR in multi-image repos that typically contain lots of 
utility images or local copies of upstream images. 

The `config-defaults.yml` file can be used to define defaults which will be inherited by the child level `<image-direcotory>/<child-dir>/config.yml` files to keep things DRY.
Supports building the same image across multiple AWS accounts and regions as well IAM cross account assume roles.

Todo:

- Add linter functionality
- Add unit tests
- Make repo public and build with go-releaser so we can consume the binaries in other pipelines

## Running

```shell
# Log level defaults to error. Image directory defaults to the current working directory
IMAGE_DIRECTORY=images LOG_LEVEL=info go run main.go run

# Validate the repo config structure
IMAGE_DIRECTORY=images LOG_LEVEL=info go run main.go lint
```
