// Copyright DataStax, Inc.
// Please see the included license file for details.

package operator

import (
	"fmt"
	"os"
	"strings"

	dockerutil "github.com/datastax/cass-operator/mage/docker"
	shutil "github.com/datastax/cass-operator/mage/sh"
	mageutil "github.com/datastax/cass-operator/mage/util"
)

const (
	envArtifactoryUser = "MO_ART_USR"
	envArtifactoryPw   = "MO_ART_PSW"
	envEcrId           = "MO_ECR_ID"
	envEcrSecret       = "MO_ECR_SECRET"
	envTags            = "MO_TAGS"
	envArtRepo         = "MO_ART_REPO"
	envEcrRepo         = "MO_ECR_REPO"
)

func dockerTag(src string, target string) {
	fmt.Println("- Re-tagging image:")
	fmt.Printf("  %s\n", src)
	fmt.Printf("  --> %s\n", target)
	dockerutil.Tag(src, target).ExecVPanic()
}

// This function is meant to simply retag a
// locally built image by adding a remote url
// to the front of it.
func retagLocalImageForRemotePush(localTag string, remoteUrl string) string {
	newTag := fmt.Sprintf("%s/%s", remoteUrl, localTag)
	dockerTag(localTag, newTag)
	return newTag
}

func retagAndPush(tags []string, remoteUrl string) {
	for _, t := range tags {
		newTag := retagLocalImageForRemotePush(strings.TrimSpace(t), remoteUrl)
		fmt.Printf("- Pushing image %s\n", newTag)
		dockerutil.Push(newTag).WithCfg(rootBuildDir).ExecVPanic()
	}
}

func awsDockerLogin(keyId string, keySecret string) {
	os.Setenv("AWS_ACCESS_KEY_ID", keyId)
	os.Setenv("AWS_SECRET_ACCESS_KEY", keySecret)
	loginStr := shutil.OutputPanic("aws", "ecr", "get-login", "--no-include-email", "--region", "us-east-1")
	args := strings.Split(loginStr, " ")
	err := dockerutil.FromArgs(args[1:len(args)]).WithCfg(rootBuildDir).ExecV()
	if err != nil {
		// Don't print the actual error message here, because it could
		// contain a valid ECR token that expires in 12 hours
		e := fmt.Errorf("Failed to login to ECR via docker cli")
		panic(e)
	}
}

// Deploy operator image to ECR.
//
// Most test workflows for Cassandra as a Service rely on operator container
// images being available from the Elastic Container Registry. We typically
// push PR builds here to enable easy testing of work prior to merge.
//
// This target assumes that you have several environment variables set:
// MO_ECR_ID - ECR access key ID
// MO_ECR_SECRET - ECR secret access key
// MO_TAGS - pipe-delimited docker tags to retag/push to ECR
func DeployToECR() {
	ecrRepo := mageutil.RequireEnv(envEcrRepo)
	keyId := mageutil.RequireEnv(envEcrId)
	keySecret := mageutil.RequireEnv(envEcrSecret)
	awsDockerLogin(keyId, keySecret)
	tags := mageutil.RequireEnv(envTags)
	retagAndPush(strings.Split(tags, "|"), ecrRepo)
}

// Deploy operator image to artifactory.
//
// Most of our internal end-to-end tests rely on operator container images
// being available from the Docker repository in Artifactory. Artifactory
// can get overloaded and so we typically push only commits to stable branches.
//
// This target assumes that you have several environment variables set:
// MO_ART_USR - artifactory user
// MO_ART_PSW - artifactory password/api key
// MO_TAGS - pipe-delimited docker tags to retag/push to Artifactory
func DeployToArtifactory() {
	artifactoryRepo := mageutil.RequireEnv(envArtRepo)
	user := mageutil.RequireEnv(envArtifactoryUser)
	pw := mageutil.RequireEnv(envArtifactoryPw)
	dockerutil.Login(rootBuildDir, user, pw, artifactoryRepo).
		WithCfg(rootBuildDir).ExecVPanic()
	tags := mageutil.RequireEnv(envTags)
	retagAndPush(strings.Split(tags, "|"), artifactoryRepo)
}
