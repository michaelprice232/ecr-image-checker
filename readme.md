# readme

## Aim

Build an app which parses the local repo for Docker config, checks whether the remote Docker tags exist in AWS ECR 
and outputs GitHub Action JSON config for any that are missing so that they can be built.

Steps:

- Setup sample GHA pipeline to validate JSON structure
- Add a configurable slog logger. Set default to ERROR
- Parse the command line parameters. Initially just "run" for the normal workflow and "lint" for the config linter
- Create a struct object which will contain the go-containerregistry and AWS clients. Use interfaces to allow for testing
- Parse top level YAML file which contains defaults and build map
- Parse all root directories and YAML files and build struct field. Merge over the top level defaults map
- Use AWS client to get ECR auth token. This is per account and region so will need to store multiple: struct field?
- Iterate through and use go-containerregistry to check if the target Docker tag is present. Store any missing ones
- Build JSON output string which will be passed as an output to the GHA job 




