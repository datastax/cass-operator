package operator

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/riptano/dse-operator/mage/util"
)

const (
	envBuiltImage      = "MO_BUILT_IMAGE"
	envArtifactoryUser = "MO_ART_USR"
	envArtifactoryPw   = "MO_ART_PSW"
	envECRUser         = "MO_ECR_USR"
	envECRPw           = "MO_ECR_PSW"
	artifactoryRepo    = "datastax-docker.jfrog.io"
	ecrRepo            = "registry.cloud-tools.datastax.com"
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

func dockerPush(path string) {
	fmt.Printf("- Pushing image %s\n", path)
	mageutil.RunV("docker", "--config", rootBuildDir, "push", path)
}

func dockerTag(src string, dest string) {
	fmt.Println("- Re-tagging image:")
	fmt.Printf("  %s\n", src)
	fmt.Printf("  --> %s\n", dest)
	mageutil.RunV("docker", "tag", src, dest)
}

func retagBuiltImage(newRepo string) string {
	builtImage := mageutil.RequireEnv(envBuiltImage)
	split := strings.Split(builtImage, ":")
	versionTag := split[len(split)-1]
	newImage := fmt.Sprintf("%s:%s", newRepo, versionTag)
	dockerTag(builtImage, newImage)
	return newImage
}

func retagAndPush(user string, pw string, repo string) {
	dockerLogin(user, pw, repo)
	newPath := fmt.Sprintf("%s/dse-operator/operator", repo)
	newImage := retagBuiltImage(newPath)
	dockerPush(newImage)
}

// Deploy operator image to ECR.
//
// Most test workflows for Cassandra as a Service rely on operator container
// images being available from the Elastic Container Registry. We typically
// push PR builds here to enable easy testing of work prior to merge.
//
// This target assumes that you have several environment variables set:
// MO_ART_USR - ECR user
// MO_ART_PSW - ECR password/api key
// MO_BUILT_IMAGE - the fully qualified image path to retag and push
func DeployToECR() {
	// Once we obtain creds, delete this code and uncomment
	// the lines below
	retagged := retagBuiltImage(ecrRepo)
	fmt.Println("Deploy to ECR is currently on hold until we get machine creds.")
	fmt.Printf("We would have normally pushed %s this run.\n", retagged)

	//user := mageutil.RequireEnv(envECRUser)
	//pw := mageutil.RequireEnv(envECRPw)
	//retagAndPush(user, pw, ecrRepo)
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
// MO_BUILT_IMAGE - the fully qualified image path to retag and push
func DeployToArtifactory() {
	// Once we obtain creds, delete this code and uncomment
	// the lines below
	retagged := retagBuiltImage(ecrRepo)
	fmt.Println("Deploy to Artifactory is currently on hold until we get machine creds.")
	fmt.Printf("We would have normally pushed %s this run.\n", retagged)

	//user := mageutil.RequireEnv(envArtifactoryUser)
	//pw := mageutil.RequireEnv(envArtifactoryPw)
	//retagAndPush(user, pw, artifactoryRepo)
}
