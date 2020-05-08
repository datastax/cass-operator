#!/usr/bin/env bash

# Expected ENV variable inputs:
#
# ECR_REPO - the ECR registry (this was misnamed in our GitHub secrets)
# GITHUB_REPO_URL - the GitHub repository url (i.e. https://github.com/datastax/cass-operator)
# GITHUB_REPO_OWNER - the owner of the repository (i.e. datastax), this is useful for forks
# GITHUB_SHA - the git SHA of the current checkout
# MO_BRANCH - set appropriately for the current branch
#
# Also, PATH should be set appropriately for Go
#

set -e

VERSION="$(mage operator:printVersion)"
FULL_VERSION="$(mage operator:printFullVersion)"
VERSION_STAMP="${GITHUB_REPO_URL}:${FULL_VERSION}"

ECR_REPOSITORY="${ECR_REPO}/datastax/cass-operator"
GH_REPOSITORY="docker.pkg.github.com/${GITHUB_REPO_OWNER}/cass-operator/operator"

# On PRs, we often checkout the sha of the branch being merged rather than
# a merge commit of the branch being merged and the branch being merged into.
# It looks like github does not change GITHUB_SHA to reflect what is actually
# checked out, so we fix that here.
GITHUB_SHA=$(git rev-parse HEAD)

ECR_TAGS=()
ECR_UBI_TAGS=()
GH_TAGS=()
GH_UBI_TAGS=()
GH_ARM64_TAGS=()

for t in "${FULL_VERSION}" "${GITHUB_SHA}" "latest"; do
  ECR_TAGS+=(--tag "${ECR_REPOSITORY}:${t}")
  ECR_UBI_TAGS+=(--tag "${ECR_REPOSITORY}:${t}-ubi7")

  GH_TAGS+=(--tag "${GH_REPOSITORY}:${t}")
  GH_UBI_TAGS+=(--tag "${GH_REPOSITORY}:${t}-ubi7")
  GH_ARM64_TAGS+=(--tag "${GH_REPOSITORY}:${t}-arm64")
done

LABELS=(
  --label "org.label-schema.schema-version=1.0"
  --label "org.label-schema.vcs-ref=$GITHUB_SHA"
  --label "org.label-schema.vcs-url=$GITHUB_REPO_URL"
  --label "org.label-schema.version=$VERSION"
)

COMMON_ARGS=(
  "${LABELS[@]}"
  --file operator/docker/base/Dockerfile
  --cache-from "type=local,src=/tmp/.buildx-cache"
  --cache-to "type=local,dest=/tmp/.buildx-cache"
)

STANDARD_ARGS=(
  "${COMMON_ARGS[@]}"
  --label "release=${VERSION_STAMP}"
  --build-arg "VERSION_STAMP=${VERSION_STAMP}"
  --target cass-operator
)

UBI_ARGS=(
  "${COMMON_ARGS[@]}"
  --label "release=${VERSION_STAMP}-ubi7"
  --build-arg "VERSION_STAMP=${VERSION_STAMP}-ubi7"
  --build-arg "BASE_OS=registry.access.redhat.com/ubi7/ubi-minimal:7.8"
  --target cass-operator-ubi
)

# Build and push standard images

docker buildx build \
  --push \
  "${STANDARD_ARGS[@]}" \
  "${ECR_TAGS[@]}" \
  --platform linux/amd64,linux/arm64 \
  .


# Build and push UBI images

docker buildx build \
  --push \
  "${UBI_ARGS[@]}" \
  "${ECR_UBI_TAGS[@]}" \
  --platform linux/amd64 \
  .


# Workaround for GH packages

docker buildx build \
  --load \
  "${STANDARD_ARGS[@]}" \
  "${GH_TAGS[@]}" \
  --platform linux/amd64 \
  .

docker buildx build \
  --load \
  "${STANDARD_ARGS[@]}" \
  "${GH_ARM64_TAGS[@]}" \
  --platform linux/arm64 \
  .

docker buildx build \
  --load \
  "${UBI_ARGS[@]}" \
  "${GH_UBI_TAGS[@]}" \
  --platform linux/amd64 \
  .

TAGS_TO_PUSH=("${GH_ARM64_TAGS[@]}" "${GH_TAGS[@]}" "${GH_UBI_TAGS[@]}")
echo "Pushing tags: " "${TAGS_TO_PUSH[@]}"

# Note: Every even index of TAGS_TO_PUSH will be the string '--tag'
#       so we skip over those while looping.

for ((x=1; x<${#TAGS_TO_PUSH[@]}; x=x+2)); do
  docker push "${TAGS_TO_PUSH[x]}"
done
