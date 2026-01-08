# readme

## Aim

Build an app which parses the local repo for Docker config, checks whether the remote Docker tags exist in AWS ECR 
and outputs GitHub Action JSON config for any that are missing so that they can be built.

Steps:

- Setup sample GHA pipeline to validate JSON structure
- Configure sample file structure locally
- Add a configurable slog logger. Set default to ERROR
- Parse the command line parameters. Initially just "run" for the normal workflow and "lint" for the config linter
- Create a struct object which will contain the AWS clients
- Parse top level YAML file which contains defaults and build map
- Parse all root directories and YAML files and build struct field. Merge over the top level defaults map
- Add 2 clients to the struct. Use interfaces to allow for testing
- Use AWS client to get AWS session and ECR auth token. This is per account and region so will need to store multiple
- Iterate through and check if the target Docker tag is present. Store any missing ones
- Build JSON output string which will be passed as an output to the GHA job 


Notes:

- Can't use multiple GHA matrix's as this results in all images being built for all accounts
- When mixing flags and position parameters the flags must go first by default
- Assume the IAM role in the app itself as there may be multiple accounts, which can't be defined cleanly in the GitHub workflow itself
- Can use the AWS client for listing Docker tags, no need for separate client


## Running

```shell
# Log level defaults to error. Image directory defaults to the current working directory

LOG_LEVEL=info go run main.go --image-directory=. run

LOG_LEVEL=info go run main.go --image-directory=. run
```
