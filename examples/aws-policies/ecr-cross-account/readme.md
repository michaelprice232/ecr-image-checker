# readme

AWS ECR private repository policy to allow cross account Docker image pushing without assuming IAM roles.
This would need to be added to each target repo, with the source account being the one the GH workflow OIDC role is based in.