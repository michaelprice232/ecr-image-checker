#!/bin/zsh

set -e

target_tag="alpine-3.20"

docker buildx build --platform linux/arm64 -t 077439031059.dkr.ecr.eu-west-1.amazonaws.com/mike-test:${target_tag} -f Dockerfile .

docker push 077439031059.dkr.ecr.eu-west-1.amazonaws.com/mike-test:${target_tag}