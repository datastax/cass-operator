// Copyright DataStax, Inc.
// Please see the included license file for details.

package images

import (
	"os"
	"strings"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	envCustomImageRegistry            = "CUSTOM_IMAGE_REGISTRY"
	envCustomImageRegistryPullSecrets = "CUSTOM_IMAGE_REGISTRY_PULL_SECRETS"
	EnvBaseImageOS                    = "BASE_IMAGE_OS"
)

// How to add new images:
//
// 1. Add a new Image enum value below
// 2. Update imageLookupMap with image url
// 3. If the image is a Cassandra/DSE image, also update maps:
//    - versionToOSSCassandra
//    - versionToDSE
// 4. Additionally, if there a is a UBI Cassandra/DSE image
//    available, also update:
//    - cassandraToUBI
//

type Image int

const (
	Cassandra_3_11_6 Image = iota
	Cassandra_3_11_7
	Cassandra_4_0_0

	UBICassandra_3_11_6
	UBICassandra_3_11_7
	UBICassandra_4_0_0

	DSE_6_8_0
	DSE_6_8_1
	DSE_6_8_2

	UBIDSE_6_8_0
	UBIDSE_6_8_1
	UBIDSE_6_8_2

	ConfigBuilder
	UBIConfigBuilder

	BusyBox
	Reaper
	BaseImageOS

	// NOTE: This line MUST be last in the const expression
	ImageEnumLength int = iota
)

var imageLookupMap map[Image]string = map[Image]string {

	Cassandra_3_11_6: "datastax/cassandra-mgmtapi-3_11_6:v0.1.5",
	Cassandra_3_11_7: "datastax/cassandra-mgmtapi-3_11_7:v0.1.12",
	Cassandra_4_0_0:  "datastax/cassandra-mgmtapi-4_0_0:v0.1.5",

	UBICassandra_3_11_6: "datastax/cassandra:3.11.6-ubi7",
	UBICassandra_3_11_7: "datastax/cassandra:3.11.7-ubi7",
	UBICassandra_4_0_0:  "datastax/cassandra:4.0-ubi7",

	DSE_6_8_0: "datastax/dse-server:6.8.0",
	DSE_6_8_1: "datastax/dse-server:6.8.1",
	DSE_6_8_2: "datastax/dse-server:6.8.2",

	UBIDSE_6_8_0: "datastax/dse-server:6.8.0-ubi7",
	UBIDSE_6_8_1: "datastax/dse-server:6.8.1-ubi7",
	UBIDSE_6_8_2: "datastax/dse-server:6.8.2-ubi7",

	ConfigBuilder:    "datastax/cass-config-builder:1.0.2",
	UBIConfigBuilder: "datastax/cass-config-builder:1.0.2-ubi7",

	BusyBox:     "busybox",
	Reaper:      "thelastpickle/cassandra-reaper:2.0.5",
}

var cassandraToUBI map[Image]Image = map[Image]Image {
	Cassandra_3_11_6: UBICassandra_3_11_6,
	Cassandra_3_11_7: UBICassandra_3_11_7,
	Cassandra_4_0_0:  UBICassandra_4_0_0,

	DSE_6_8_0: UBIDSE_6_8_0,
	DSE_6_8_1: UBIDSE_6_8_1,
	DSE_6_8_2: UBIDSE_6_8_2,
}

var versionToOSSCassandra map[string]Image = map[string]Image {
	"3.11.6": Cassandra_3_11_6,
	"3.11.7": Cassandra_3_11_7,
	"4.0.0":  Cassandra_4_0_0,
}

var versionToDSE map[string]Image = map[string]Image {
	"6.8.0": DSE_6_8_0,
	"6.8.1": DSE_6_8_1,
	"6.8.2": DSE_6_8_2,
}

var log = logf.Log.WithName("images")

func getCustomImageRegistry() string {
	return os.Getenv(envCustomImageRegistry)
}

func stripRegistry(image string) string {
	comps := strings.Split(image, "/")

	if len(comps) > 1 && strings.Contains(comps[0], ".") || strings.Contains(comps[0], ":") {
		return strings.Join(comps[1:], "/")
	} else {
		return image
	}
}

func applyCustomRegistry(image string) string {
	customRegistry := getCustomImageRegistry()
	customRegistry = strings.TrimSuffix(customRegistry, "/")

	if customRegistry == "" {
		return image
	} else {
		imageNoRegistry := stripRegistry(image)
		return fmt.Sprintf("%s/%s", customRegistry, imageNoRegistry)
	}
}

func GetImage(name Image) string {
	image, ok := imageLookupMap[name]
	if !ok {
		if name == BaseImageOS {
			image = os.Getenv(EnvBaseImageOS)
		} else {
			// This should never happen as we have a unit test
			// to ensure imageLookupMap is fully populated.
			log.Error(nil, "Could not find image", "image", int(name))
			return ""
		}
	}

	if image != "" {
		return applyCustomRegistry(image)
	} else {
		return ""
	}
}

func (image Image) String() string {
	return GetImage(image)
}

func getCassandraUBIImage(name Image) (Image, error) {
	for key, value := range cassandraToUBI {
		// We check against both the key and the value as the 
		// passed name may already be a UBI image
		if value == name || key == name {
			return value, nil
		}
	}

	// If we got here, then we didn't find anything
	return -1, fmt.Errorf("Could not find a UBI image for %s", name)
}

func shouldUseUBI() bool {
	baseImageOs := os.Getenv(EnvBaseImageOS)
	return baseImageOs != ""
}

func GetCassandraImage(serverType, version string) (string, error) {
	var imageKey Image
	var found bool

	switch serverType {
	case "dse":
		imageKey, found = versionToDSE[version]
	case "cassandra":
		imageKey, found = versionToOSSCassandra[version]
	default:
		return "", fmt.Errorf("Unknown server type '%s'", serverType)
	}

	if !found {
		return "", fmt.Errorf("server '%s' and version '%s' do not work together", serverType, version)
	}

	var err error
	if shouldUseUBI() {
		imageKey, err = getCassandraUBIImage(imageKey)
		if err != nil {
			return "", fmt.Errorf("Coud not find UBI image for server '%s' and version '%s': %w", serverType, version, err)
		}
	}

	return GetImage(imageKey), nil
}

func GetConfigBuilderImage() string {
	if shouldUseUBI() {
		return GetImage(UBIConfigBuilder)
	} else {
		return GetImage(ConfigBuilder)
	}
}

func GetReaperImage() string {
	return GetImage(Reaper)
}

func GetSystemLoggerImage() string {
	if shouldUseUBI() {
		return GetImage(BaseImageOS)
	} else {
		return GetImage(BusyBox)
	}
}

func AddCustomRegistryImagePullSecrets(podSpec *corev1.PodSpec) bool {
	secretName := os.Getenv(envCustomImageRegistryPullSecrets)
	if secretName != "" {
		podSpec.ImagePullSecrets = append(
			podSpec.ImagePullSecrets, 
			corev1.LocalObjectReference{Name: secretName})
		return true
	}
	return false
}