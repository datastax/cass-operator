package operator

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/magefile/mage/sh"
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

func dockerLogin(user string, pw string, repo string) {
	fmt.Printf("- Logging into repo %s\n", repo)
	cmd := exec.Command(
		"docker", "--config", rootBuildDir,
		"login", "-u", user, "--password-stdin", repo)

	buffer := bytes.Buffer{}
	buffer.Write([]byte(pw))
	cmd.Stdin = &buffer

	out, err := cmd.Output()
	if msg := string(out); msg != "" {
		fmt.Println(msg)
	}
	mageutil.PanicOnError(err)
}

func dockerPanic(args ...string) {
	argz := []string{"--config", rootBuildDir}
	argz = append(argz, args...)
	mageutil.RunV("docker", argz...)
}

func docker(args ...string) error {
	argz := []string{"--config", rootBuildDir}
	argz = append(argz, args...)
	return sh.RunV("docker", argz...)
}

func dockerPush(path string) {
	fmt.Printf("- Pushing image %s\n", path)
	dockerPanic("push", path)
}

func dockerTag(src string, dest string) {
	fmt.Println("- Re-tagging image:")
	fmt.Printf("  %s\n", src)
	fmt.Printf("  --> %s\n", dest)
	dockerPanic("tag", src, dest)
}

func retagImage(currentTag string, newRepo string) string {
	newPath := fmt.Sprintf("%s/dse-operator/operator", newRepo)

	// We just want to grab the version part of the tag and discard
	// the old repo path
	split := strings.Split(currentTag, ":")
	versionTag := split[len(split)-1]

	newImage := fmt.Sprintf("%s:%s", newPath, versionTag)
	dockerTag(currentTag, newImage)
	return newImage
}

func retagAndPush(tags []string, newRepo string) {
	for _, t := range tags {
		newTag := retagImage(strings.TrimSpace(t), newRepo)
		dockerPush(newTag)
	}
}

func awsDockerLogin(keyId string, keySecret string) {
	os.Setenv("AWS_ACCESS_KEY_ID", keyId)
	os.Setenv("AWS_SECRET_ACCESS_KEY", keySecret)
	loginStr := mageutil.Output("aws", "ecr", "get-login", "--no-include-email", "--region", "us-east-1")
	args := strings.Split(loginStr, " ")
	err := docker(args[1:len(args)]...)
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
	dockerLogin(user, pw, artifactoryRepo)
	tags := mageutil.RequireEnv(envTags)
	retagAndPush(strings.Split(tags, "|"), artifactoryRepo)
}
