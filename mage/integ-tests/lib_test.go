// Copyright DataStax, Inc.
// Please see the included license file for details.

package integutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func tempSetEnv(name, value string) (func(), error) {
	oldValue, wasDefined := os.LookupEnv(name)
	restore := func() {
		if !wasDefined {
			_ = os.Unsetenv(name)
		} else {
			_ = os.Setenv(name, oldValue)
		}
	}

	err := os.Setenv(name, value)
	return restore, err
}

func Test_GetTestTypeDse(t *testing.T) {
	testDir := "some_test"
	testType := getTestType(testDir)
	assert.Equal(t, DSE, testType)
}

func Test_GetTestTypeUbiDse(t *testing.T) {
	testDir := "some_test_ubi"
	testType := getTestType(testDir)
	assert.Equal(t, UBI_DSE, testType)
}

func Test_GetTestTypeOss(t *testing.T) {
	testDir := "some_oss_testing"
	testType := getTestType(testDir)
	assert.Equal(t, OSS, testType)
}

func Test_GetTestTypeUbiOss(t *testing.T) {
	testDir := "some_ubi_oss_test"
	testType := getTestType(testDir)
	assert.Equal(t, UBI_OSS, testType)
}
