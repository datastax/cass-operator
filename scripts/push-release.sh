#!/usr/bin/env bash

set -e

# Expected ENV variable inputs:
#
# ECR_REPO - the ECR registry (this was misnamed in our GitHub secrets)
# GITHUB_REF - the git ref of the tag
# GITHUB_SHA - the git SHA of the current checkout
#

GIT_TAG="${GITHUB_REF##*/}"
VERSION=${GIT_TAG#v} # strip the initial 'v' from the tag to get the version

DOCKERHUB_REPOSITORY="datastax/cass-operator"
ECR_REPOSITORY="${ECR_REPO}/datastax/cass-operator"
REDHAT_REPOSITORY="${REDHAT_REPO}/cass-operator"

# Get the version label of the ECR image so that we can double
# check that it makes sense for this tag name.
VERSION_PATH='.Labels["org.label-schema.version"]'
LABEL_VERSION="$(skopeo inspect "docker://${ECR_REPOSITORY}:${GITHUB_SHA}" | jq "$VERSION_PATH" --raw-output)"
LABEL_VERSION_UBI="$(skopeo inspect "docker://${ECR_REPOSITORY}:${GITHUB_SHA}-ubi7" | jq "$VERSION_PATH" --raw-output)"

# Sanity check. This should never happen.
if ! [ "$LABEL_VERSION" = "$LABEL_VERSION_UBI" ]; then
  echo "Standard and UBI images were not labeled with the same version"
  exit 1
fi

# Ensure the image has a version appropriate for this tag
# to prevent confusion.
#
# There are two checks in the following if-statement. The
# first handles the case of a standard release where
# LABEL_VERSION will contain a "-release" suffix that we
# generally do not include in the tag. The second handles
# the case of non-standard releases (a release candidate,
# alpha, etc.) where we _would_ expect to see the relevant
# suffix in the tag name.
if ! [ "v${LABEL_VERSION}" = "${GIT_TAG}-release" ] && ! [ "v${LABEL_VERSION}" = "${GIT_TAG}" ]; then
  echo "Git tag $GIT_TAG does not align with version number ${LABEL_VERSION}"
  exit 1
fi

# Tag images for DockerHub and push them
for t in "$VERSION" "latest"; do
  skopeo copy --all "docker://${ECR_REPOSITORY}:${GITHUB_SHA}" "docker://${DOCKERHUB_REPOSITORY}:${t}"
  skopeo copy --all "docker://${ECR_REPOSITORY}:${GITHUB_SHA}-ubi7" "docker://${DOCKERHUB_REPOSITORY}:${t}-ubi7"
done

skopeo copy --all "docker://${ECR_REPOSITORY}:${GITHUB_SHA}-ubi7" "docker://${REDHAT_REPOSITORY}:${VERSION}-ubi7"
