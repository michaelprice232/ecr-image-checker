#!/bin/zsh

set -e

target_tag="my-tag"

docker buildx build --platform linux/arm64 -t 077439031059.dkr.ecr.ca-central-1.amazonaws.com/mike-another-region-test:${target_tag} -f Dockerfile .

docker push  077439031059.dkr.ecr.ca-central-1.amazonaws.com/mike-another-region-test:${target_tag}