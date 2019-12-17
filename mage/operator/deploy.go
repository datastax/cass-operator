package operator

import (
	"fmt"
	"os"
	"strings"

	"github.com/riptano/dse-operator/mage/docker"
	"github.com/riptano/dse-operator/mage/sh"
	"github.com/riptano/dse-operator/mage/util"
)

const (
	envArtifactoryUser = "MO_ART_USR"
	envArtifactoryPw   = "MO_ART_PSW"
	envEcrId           = "MO_ECR_ID"
	envEcrSecret       = "MO_ECR_SECRET"
	envTags            = "MO_TAGS"
	artifactoryRepo    = "datastax-docker.jfrog.io"
	ecrRepo            = "237073351946.dkr.ecr.us-east-1.amazonaws.com"
)

func dockerTag(src string, target string) {
	fmt.Println("- Re-tagging image:")
	fmt.Printf("  %s\n", src)
	fmt.Printf("  --> %s\n", target)
	dockerutil.Tag(src, target).ExecVPanic()
}

func retagImage(currentTag string, newRepo string) string {
	newPath := fmt.Sprintf("%s/dse-operator/operator", newRepo)

	// We just want to grab the version part of the tag and discard
	// the old repo path
	// For example:
	// datastax/dse-operator:someversion
	// ^ only keep 'someversion'
	// so that we can put a new repo path in front of it
	split := strings.Split(currentTag, ":")
	versionTag := split[len(split)-1]

	newImage := fmt.Sprintf("%s:%s", newPath, versionTag)
	dockerTag(currentTag, newImage)
	return newImage
}

func retagAndPush(tags []string, newRepo string) {
	for _, t := range tags {
		newTag := retagImage(strings.TrimSpace(t), newRepo)
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
	user := mageutil.RequireEnv(envArtifactoryUser)
	pw := mageutil.RequireEnv(envArtifactoryPw)
	dockerutil.Login(rootBuildDir, user, pw, artifactoryRepo).
		WithCfg(rootBuildDir).ExecVPanic()
	tags := mageutil.RequireEnv(envTags)
	retagAndPush(strings.Split(tags, "|"), artifactoryRepo)
}
