// Copyright DataStax, Inc.
// Please see the included license file for details.

package images

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
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

func Test_AllImageEnumValuesHaveImageDefined(t *testing.T) {
	for i := 0; i < ImageEnumLength; i++ {
		if Image(i) == BaseImageOS {
			// the BaseImageOS is unique in that we get it's value from an
			// environment variable, so if the environment variable is not
			// defined, then we will have no value here.
			continue
		}

		assert.NotEmpty(t, GetImage(Image(i)), "No image defined for Image enum value %d", i)
	}
}

func Test_BaseImageOS(t *testing.T) {
	restore, err := tempSetEnv(EnvBaseImageOS, "my-test-value")
	require.NoError(t, err)
	defer restore()

	assert.Equal(t, "my-test-value", GetImage(BaseImageOS))
}

func Test_DefaultRegistryOverride(t *testing.T) {
	restore, err := tempSetEnv(envDefaultRegistryOverride, "localhost:5000")
	require.NoError(t, err)
	defer restore()

	image := GetConfigBuilderImage()
	assert.True(t, strings.HasPrefix(image, "localhost:5000/"))
}

func Test_CalculateDockerImageRunsAsCassandra(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{
			version: "3.11.6",
			want:    false,
		},
		{
			version: "3.11.7",
			want:    false,
		},
		{
			version: "4.0.0",
			want:    false,
		},
		// We default to true
		{
			version: "4.0.1",
			want:    true,
		},
	}
	for _, tt := range tests {
		got := CalculateDockerImageRunsAsCassandra(tt.version)

		assert.Equal(t, got, tt.want, fmt.Sprintf("Version: %s should not have returned %v", tt.version, got))
	}
}

func Test_IsOssVersionSupported(t *testing.T) {
	tests := []struct {
		version  string
		expected bool
	}{
		{
			version:  "3.11.6",
			expected: true,
		},
		{
			version:  "3.11.10",
			expected: true,
		},
		{
			version:  "3.0.23",
			expected: false,
		},
		{
			version:  "4.0.0",
			expected: true,
		},
		{
			version:  "4.0.1",
			expected: true,
		},
		{
			version:  "4.0-beta4",
			expected: true,
		},
		{
			version:  "4.1.0",
			expected: false,
		},
	}
	for _, tt := range tests {
		supported := IsOssVersionSupported(tt.version)

		assert.Equal(t, supported, tt.expected, fmt.Sprintf("Version: %s should not have returned supported=%v", tt.version, supported))
	}
}
