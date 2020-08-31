// Copyright DataStax, Inc.
// Please see the included license file for details.

package images

import (
	"os"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	for i :=0; i < ImageEnumLength; i++ {
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

func Test_CustomRegistry(t *testing.T) {
	restore, err := tempSetEnv(envCustomImageRegistry, "localhost:5000")
	require.NoError(t, err)
	defer restore()

	image := GetConfigBuilderImage()
	assert.True(t, strings.HasPrefix(image, "localhost:5000/"))
}