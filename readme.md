# readme

## Aim

Build an app which parses the local repo for Docker config, checks whether the remote Docker tags exist in AWS ECR 
and outputs GitHub Action JSON config for any that are missing so that they can be built.

Todo:

- Update YAMl schema to support multiple accounts and regions per image
- Add linter functionality
- Add unit tests
- Make repo public and build with go-releaser so we can consume the binaries in other pipelines


Notes:

- Can't use multiple GHA matrix's as this results in all images being built for all accounts
- When mixing flags and position parameters the flags must go first by default
- Assume the IAM role in the app itself as there may be multiple accounts, which can't be defined cleanly in the GitHub workflow itself
- Can use the AWS client for listing Docker tags, no need for separate client
- Don't add datetime suffix functionality otherwise there will be continual builds, which goes against the aim of this setup


## Running

```shell
# Log level defaults to error. Image directory defaults to the current working directory

LOG_LEVEL=info go run main.go --image-directory=. run

LOG_LEVEL=info go run main.go --image-directory=. run
```
