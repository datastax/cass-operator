package cfgutil

import (
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/datastax/cass-operator/mage/util"
	"gopkg.in/yaml.v2"
)

const (
	buildSettings = "buildsettings.yaml"
)

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
	DseImage           string `yaml:"dseImage"`
	ConfigBuilderImage string `yaml:"configBuilderImage"`
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
