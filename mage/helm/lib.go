// Copyright DataStax, Inc.
// Please see the included license file for details.

package helm_util

import (
	"fmt"

	shutil "github.com/datastax/cass-operator/mage/sh"
)

func Install(chartPath string, releaseName string, namespace string, overrides map[string]string) error {
	args := []string{
		"install",
		fmt.Sprintf("--namespace=%s", namespace),
	}

	if overrides != nil && len(overrides) > 0 {
		var overrideString = ""
		for key, val := range overrides {
			if overrideString == "" {
				overrideString = fmt.Sprintf("%s=%s", key, val)
			} else {

				overrideString = fmt.Sprintf("%s,%s=%s", overrideString, key, val)
			}
		}

		args = append(args, "--set", overrideString)
	}

	args = append(args, releaseName, chartPath)
	return shutil.RunV("helm", args...)
}

func Uninstall(releaseName string, namespace string) error {
	args := []string{
		"uninstall",
		fmt.Sprintf("--namespace=%s", namespace),
		releaseName,
	}
	return shutil.RunV("helm", args...)
}
