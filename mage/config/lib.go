// Copyright DataStax, Inc.
// Please see the included license file for details.

package cfgutil

import (
	"fmt"
	"os"
	"io/ioutil"
	"regexp"

	mageutil "github.com/datastax/cass-operator/mage/util"
	"gopkg.in/yaml.v2"
)

const (
	buildSettings        = "buildsettings.yaml"
	defaultOperatorImage = "datastax/cass-operator:latest"
	envOperatorImage     = "M_OPERATOR_IMAGE"
)

func GetOperatorImage() string {
	image := os.Getenv(envOperatorImage)
	if "" == image {
		return defaultOperatorImage
	}
	return image
}

type Version struct {
	Major      int    `yaml:"major"`
	Minor      int    `yaml:"minor"`
	Patch      int    `yaml:"patch"`
	Prerelease string `yaml:"prerelease"`
}

func (v Version) String() string {
	version := fmt.Sprintf("%v.%v.%v", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		pre := EnsureAlphaNumericDash(v.Prerelease)
		version = fmt.Sprintf("%v-%v", version, pre)
	}
	return version
}

func EnsureAlphaNumericDash(str string) string {
	r := regexp.MustCompile("[^A-z0-9\\-]")
	return r.ReplaceAllString(str, "-")
}

type Master struct {
	Plugins []string `yaml:"plugins"`
}

type Jenkins struct {
	Master Master `yaml:"master"`
}

type Dev struct {
	Images []string `yaml:"images"`
}

type BuildSettings struct {
	Version Version `yaml:"version"`
	Jenkins Jenkins `yaml:"jenkins"`
	Dev     Dev     `yaml:"dev"`
}

func ReadBuildSettings() BuildSettings {
	var settings BuildSettings
	d, err := ioutil.ReadFile(buildSettings)
	mageutil.PanicOnError(err)

	err = yaml.Unmarshal(d, &settings)
	mageutil.PanicOnError(err)

	return settings
}

type ClusterActions struct {
	DeleteCluster       func() error
	ClusterExists       func() bool
	CreateCluster       func()
	LoadImage           func(image string)
	InstallTool         func()
	ReloadLocalImage    func(image string)
	ApplyDefaultStorage func()
	SetupKubeconfig     func()
	DescribeEnv         func() map[string]string
}

func NewClusterActions(
	deleteF func() error,
	clusterExistsF func() bool,
	createClusterF func(),
	loadImageF func(string),
	installToolF func(),
	reloadLocalImageF func(string),
	applyDefaultStorageF func(),
	setupKubeconfigF func(),
	describeEnvF func() map[string]string,
) ClusterActions {
	return ClusterActions{
		DeleteCluster:       deleteF,
		ClusterExists:       clusterExistsF,
		CreateCluster:       createClusterF,
		LoadImage:           loadImageF,
		InstallTool:         installToolF,
		ReloadLocalImage:    reloadLocalImageF,
		ApplyDefaultStorage: applyDefaultStorageF,
		SetupKubeconfig:     setupKubeconfigF,
		DescribeEnv:         describeEnvF,
	}
}
