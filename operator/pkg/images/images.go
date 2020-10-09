// Copyright DataStax, Inc.
// Please see the included license file for details.

package images

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	envDefaultRegistryOverride            = "DEFAULT_CONTAINER_REGISTRY_OVERRIDE"
	envDefaultRegistryOverridePullSecrets = "DEFAULT_CONTAINER_REGISTRY_OVERRIDE_PULL_SECRETS"
	EnvBaseImageOS                        = "BASE_IMAGE_OS"
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
//    - versionToUBIOSSCassandra
//    - versionToUBIDSE
//

type Image int

// IMPORTANT: Only Image enum values (and ImageEnumLength) should go in this
// const expression
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
	DSE_6_8_3
	DSE_6_8_4

	UBIDSE_6_8_0
	UBIDSE_6_8_1
	UBIDSE_6_8_2
	UBIDSE_6_8_3
	UBIDSE_6_8_4

	ConfigBuilder
	UBIConfigBuilder

	BusyBox
	Reaper
	BaseImageOS

	// NOTE: This line MUST be last in the const expression
	ImageEnumLength int = iota
)

var imageLookupMap map[Image]string = map[Image]string{

	Cassandra_3_11_6: "datastax/cassandra-mgmtapi-3_11_6:v0.1.5",
	Cassandra_3_11_7: "datastax/cassandra-mgmtapi-3_11_7:v0.1.13",
	Cassandra_4_0_0:  "datastax/cassandra-mgmtapi-4_0_0:v0.1.12",

	UBICassandra_3_11_6: "datastax/cassandra:3.11.6-ubi7",
	UBICassandra_3_11_7: "datastax/cassandra:3.11.7-ubi7",
	UBICassandra_4_0_0:  "datastax/cassandra:4.0-ubi7",

	DSE_6_8_0: "datastax/dse-server:6.8.0",
	DSE_6_8_1: "datastax/dse-server:6.8.1",
	DSE_6_8_2: "datastax/dse-server:6.8.2",
	DSE_6_8_3: "datastax/dse-server:6.8.3",
	DSE_6_8_4: "datastax/dse-server:6.8.4",

	UBIDSE_6_8_0: "datastax/dse-server:6.8.0-ubi7",
	UBIDSE_6_8_1: "datastax/dse-server:6.8.1-ubi7",
	UBIDSE_6_8_2: "datastax/dse-server:6.8.2-ubi7",
	UBIDSE_6_8_3: "datastax/dse-server:6.8.3-ubi7",
	UBIDSE_6_8_4: "datastax/dse-server:6.8.4-ubi7",

	ConfigBuilder:    "datastax/cass-config-builder:1.0.3",
	UBIConfigBuilder: "datastax/cass-config-builder:1.0.3-ubi7",

	BusyBox: "busybox",
	Reaper:  "thelastpickle/cassandra-reaper:2.0.5",
}

var versionToOSSCassandra map[string]Image = map[string]Image{
	"3.11.6": Cassandra_3_11_6,
	"3.11.7": Cassandra_3_11_7,
	"4.0.0":  Cassandra_4_0_0,
}

var versionToUBIOSSCassandra map[string]Image = map[string]Image{
	"3.11.6": UBICassandra_3_11_6,
	"3.11.7": UBICassandra_3_11_7,
	"4.0.0":  UBICassandra_4_0_0,
}

var versionToDSE map[string]Image = map[string]Image{
	"6.8.0": DSE_6_8_0,
	"6.8.1": DSE_6_8_1,
	"6.8.2": DSE_6_8_2,
	"6.8.3": DSE_6_8_3,
	"6.8.4": DSE_6_8_4,
}

var versionToUBIDSE map[string]Image = map[string]Image{
	"6.8.0": UBIDSE_6_8_0,
	"6.8.1": UBIDSE_6_8_1,
	"6.8.2": UBIDSE_6_8_2,
	"6.8.3": UBIDSE_6_8_3,
	"6.8.4": UBIDSE_6_8_4,
}

var log = logf.Log.WithName("images")

func stripRegistry(image string) string {
	comps := strings.Split(image, "/")

	if len(comps) > 1 && strings.Contains(comps[0], ".") || strings.Contains(comps[0], ":") {
		return strings.Join(comps[1:], "/")
	} else {
		return image
	}
}

func applyDefaultRegistryOverride(image string) string {
	customRegistry := os.Getenv(envDefaultRegistryOverride)
	customRegistry = strings.TrimSuffix(customRegistry, "/")

	if customRegistry == "" {
		return image
	} else {
		imageNoRegistry := stripRegistry(image)
		return fmt.Sprintf("%s/%s", customRegistry, imageNoRegistry)
	}
}

// Does this Docker Image run as the cassandra user?
func DockerImageRunsAsCassandra(version string, image string) bool {

	// The ubi versions of these images are assumed to always run as root,
	// because we cannot see the mgmt api version
	if shouldUseUBI() && (version == "3.11.6" || version == "3.11.7" || version == "4.0.0") {
		return false
	}

	// Any version of the management api that would be too old is going to start with 0.1
	// Therefore we can simply examine the patch version of the mgmt api

	re := regexp.MustCompile(`:v0.1.(.*)$`)
	matches := re.FindSubmatch([]byte(image))

	// Default to false if the image name is not parseable
	if len(matches) < 2 {
		return false
	}

	patchVersion, _ := strconv.Atoi(string(matches[1]))

	if version == "3.11.6" && patchVersion > 5 {
		return true
	} else if version == "3.11.7" && patchVersion > 13 {
		return true
	} else if version == "4.0.0" && patchVersion > 12 {
		return true
	}

	return false
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
		return applyDefaultRegistryOverride(image)
	} else {
		return ""
	}
}

func (image Image) String() string {
	return GetImage(image)
}

func shouldUseUBI() bool {
	baseImageOs := os.Getenv(EnvBaseImageOS)
	return baseImageOs != ""
}

func GetCassandraImage(serverType, version string) (string, error) {
	var imageKey Image
	var found bool

	dseMap := versionToDSE
	cassandraMap := versionToOSSCassandra

	if shouldUseUBI() {
		dseMap = versionToUBIDSE
		cassandraMap = versionToUBIOSSCassandra
	}

	switch serverType {
	case "dse":
		imageKey, found = dseMap[version]
	case "cassandra":
		imageKey, found = cassandraMap[version]
	default:
		return "", fmt.Errorf("Unknown server type '%s'", serverType)
	}

	if !found {
		return "", fmt.Errorf("server '%s' and version '%s' do not work together", serverType, version)
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

func AddDefaultRegistryImagePullSecrets(podSpec *corev1.PodSpec) bool {
	secretName := os.Getenv(envDefaultRegistryOverridePullSecrets)
	if secretName != "" {
		podSpec.ImagePullSecrets = append(
			podSpec.ImagePullSecrets,
			corev1.LocalObjectReference{Name: secretName})
		return true
	}
	return false
}
