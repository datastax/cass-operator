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

func Test_DockerImageRunsAsCassandra(t *testing.T) {
	tests := []struct {
		version string
		image   string
		want    bool
		ubi     bool
	}{
		{
			version: "3.11.6",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.5",
			want:    false,
			ubi:     false,
		},
		{
			version: "3.11.7",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.13",
			want:    false,
			ubi:     false,
		},
		{
			version: "4.0.0",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.12",
			want:    false,
			ubi:     false,
		},
		{
			version: "3.11.6",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.6",
			want:    true,
			ubi:     false,
		},
		{
			version: "3.11.7",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.14",
			want:    true,
			ubi:     false,
		},
		{
			version: "4.0.0",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.13",
			want:    true,
			ubi:     false,
		},
		// Ubi skips the image check
		{
			version: "3.11.6",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.500",
			want:    false,
			ubi:     true,
		},
		{
			version: "3.11.7",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.130",
			want:    false,
			ubi:     true,
		},
		{
			version: "4.0.0",
			image:   "datastax/cassandra-mgmtapi-3_11_6:v0.1.120",
			want:    false,
			ubi:     true,
		},
	}
	for _, tt := range tests {
		got := false
		if tt.ubi {
			restore, err := tempSetEnv(EnvBaseImageOS, "something")
			require.NoError(t, err)
			got = DockerImageRunsAsCassandra(tt.version, tt.image)
			restore()
		} else {
			got = DockerImageRunsAsCassandra(tt.version, tt.image)
		}

		assert.Equal(t, got, tt.want, fmt.Sprintf("Version: %s and Image: %s should not have returned %v", tt.version, tt.image, got))
	}
}
