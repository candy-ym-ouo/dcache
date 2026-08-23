#!/bin/bash
set -e
IMAGE_NAME=${1:-dcache}
DOCKER_PLATFORM=${2:-linux/amd64}
docker build --platform "$DOCKER_PLATFORM" -f benzhi.Dockerfile -t "$IMAGE_NAME" .
printf '\nDocker image %s built successfully!\n' "$IMAGE_NAME"
