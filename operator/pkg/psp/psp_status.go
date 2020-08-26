// Copyright DataStax, Inc.
// Please see the included license file for details.

package psp

import (
	"fmt"
	"strings"
	"context"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"github.com/datastax/cass-operator/operator/pkg/utils"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/api/errors"
	"gopkg.in/yaml.v2"
)

type HealthStatusUpdater interface {
	Update(dc api.CassandraDatacenter) error
}

type NoOpUpdater struct {}

func (*NoOpUpdater) Update(dc api.CassandraDatacenter) error {
	return nil
}

type HealthStatusUpdaterImpl struct {
	dao DAO
}

func NewHealthStatusUpdater(client runtimeClient.Client, namespace string) HealthStatusUpdater {
	return &HealthStatusUpdaterImpl{
		dao: NewDao(client, namespace),
	}
}

func (updater *HealthStatusUpdaterImpl) Update(dc api.CassandraDatacenter) error {
	// Update the catalog config map
	// Even though the catalog is generally static, it might change due to upgrading the
	// operator
	catalog := buildCatalog()
	err := updater.dao.UpsertCatalog(catalog)
	if err != nil {
		return err
	}

	// Update the health status config map
	// Keep in mind that this config map is shared between _all_ the CassandraDatacenter
	// instances
	health, err := updater.dao.GetHealthData()
	if err != nil {
		return err
	}

	updateHealth(health, dc)

	return updater.dao.UpsertHealthData(health)
}

type DAO interface {
	GetHealthData() (*Health, error)
	UpsertHealthData(health *Health) error
	UpsertCatalog(catalog Catalog) error
}

type DAOImpl struct {
	catalogNamespacedName types.NamespacedName
	healthChecksNamespacedName types.NamespacedName
	client runtimeClient.Client
}

func NewDao(client runtimeClient.Client, namespace string) DAO {
	return &DAOImpl{
		client: client,
		healthChecksNamespacedName: types.NamespacedName{
			Name: "health-check-0",
			Namespace: namespace,
		},
		catalogNamespacedName: types.NamespacedName{
			Name: "health-catalog-0",
			Namespace: namespace,
		},
	}
}

func (dao *DAOImpl) GetHealthData() (*Health, error) {
	configMap, err := getConfigMap(dao.client, dao.healthChecksNamespacedName)
	if err != nil {
		return nil, err
	}

	if configMap == nil {
		return &Health{}, nil
	}

	return loadHealthFromConfigMap(configMap)
}

func (dao *DAOImpl) UpsertHealthData(health *Health) error {
	configMap := createHealthCheckConfigMap(health, dao.healthChecksNamespacedName)
	return upsertConfigMap(dao.client, configMap)
}

func (dao *DAOImpl) UpsertCatalog(catalog Catalog) error {
	configMap := createCatalogConfigMap(catalog, dao.catalogNamespacedName)
	return upsertConfigMap(dao.client, configMap)
}

const (
	HealthLabel             = "vmware.vsphere.health"
	HealthLabelCatalogValue = "catalog"
	HealthLabelHealthValue  = "health"
)

type Spec map[string]interface{}

type HealthStatus string

const (
	HealthGreen  HealthStatus = "green"
	HealthYellow HealthStatus = "yellow"
	HealthRed    HealthStatus = "red"
)

type InstanceHealth struct {
	Instance  string       `json:"instance"`
	Namespace string       `json:"namespace"`
	Health    HealthStatus `json:"health"`
}

type Status struct {
	InstanceHealth []InstanceHealth `json:"instanceHealth"`
}

type Health struct {
	Spec `json:"spec"`
	Status `json:"status"`
}

func instanceHealthSchema() map[string]interface{}{
	return map[string]interface{}{
		"name": "instanceHealth",
		"fields": []map[string]interface{}{
			map[string]interface{}{
				"name": "instance",
				"type": "string",
			},
			map[string]interface{}{
				"name": "namespace",
				"type": "string",
			},
			map[string]interface{}{
				"name": "health",
				"type": "string",
			},
		},
	}
}

func buildSchema() map[string]interface{} {
	// In a good and just world, we'd generate most of this via reflection on
	// the Health/InstanceHealth structs, but lets get something that at least
	// works before we go all fancy pants. As it happens, at the end of the 
	// day, this is just json, and you know what looks a lot like json? strings,
	// lists, and maps. Good thing Go gives us all of those out of the box.
	return map[string]interface{}{
		"name": "CassandraDatacenter",
		"healthchecks": []map[string]interface{}{
			instanceHealthSchema(),
		},
	}
}

func createInstanceHealth(dc api.CassandraDatacenter) InstanceHealth {
	// Everything is happiness and rainbows until proven otherwise
	status := HealthGreen 

	if corev1.ConditionFalse == dc.GetConditionStatus(api.DatacenterReady) || corev1.ConditionTrue == dc.GetConditionStatus(api.DatacenterResuming) {
		status = HealthRed
	} else {
		// Okay, we are ready, but we might be in some sort of degraded state too
		degradedWhenTrue := []api.DatacenterConditionType{
			api.DatacenterReplacingNodes,
			api.DatacenterScalingUp,
			api.DatacenterUpdating,
			api.DatacenterResuming,
			api.DatacenterRollingRestart,
		}
		for _, condition := range degradedWhenTrue {
			if corev1.ConditionTrue == dc.GetConditionStatus(condition) {
				status = HealthYellow
				break
			}
		}
	}

	health := InstanceHealth{
		Instance: dc.Name,
		Namespace: dc.Namespace,
		Health: status,
	}

	return health
}

func updateHealth(health *Health, dc api.CassandraDatacenter) {
	health.Spec = buildSchema()
	instanceHealths := health.Status.InstanceHealth
	index := -1
	for i, _ := range instanceHealths {
		if instanceHealths[i].Instance == dc.Name && instanceHealths[i].Namespace == dc.Namespace {
			index = i
			break
		}
	}
	instanceHealth := createInstanceHealth(dc)
	if index > -1 {
		instanceHealths[index] = instanceHealth
	} else {
		instanceHealths = append(instanceHealths, instanceHealth)
	}

	health.Status.InstanceHealth = instanceHealths
}


func loadHealthFromConfigMap(configMap *corev1.ConfigMap) (*Health, error) {
	health := &Health{}
	configRaw, ok := configMap.BinaryData["health"]
	if !ok {
		return health, nil
	} else {
		err := yaml.Unmarshal(configRaw, health)
		return health, err
	}
}

func createHealthCheckConfigMap(health *Health, healthCheckNamespacedName types.NamespacedName) *corev1.ConfigMap {
	configMap := newConfigMap(healthCheckNamespacedName)
	saveHealthCheckToConfigMap(health, configMap)
	utils.AddHashAnnotation(configMap)
	configMap.SetLabels(
		utils.MergeMap(
			map[string]string{}, 
			configMap.GetLabels(), 
			map[string]string{HealthLabel: HealthLabelHealthValue}))

	return configMap
}

func saveHealthCheckToConfigMap(health *Health, configMap *corev1.ConfigMap) error {
	healthRaw, err := yaml.Marshal(health)
	if err != nil {
		return err
	}

	configMap.Data["health"] = string(healthRaw)

	return nil
}

type Catalog map[string]string

func prefixKeys(m map[string]string, prefix string) map[string]string {
	n := map[string]string{}
	for k, v := range m {
		n[fmt.Sprintf("%s%s", prefix, k)] = v
	}
	return n
}

func buildInstanceHealthCatalog() map[string]string {
	desc := strings.Join([]string{
		"Provides the overall health of each Cassandra datacenter. A value ",
		"of 'green' means the datacenter is available and all of its nodes ", 
		"are in good working order. A value of 'yellow' means that the ",
		"datacenter is available, but some node may be down (e.g. there ",
		"might be a rolling restart in progress). A value of 'red' means ",
		"the cluster is not available (e.g. several nodes are down).",
	}, "")
	return map[string]string {
		"testname": "Instance Health",
		"short": "Health of cassandra datacenter",
		"desc": desc,
		"table.label": "CassandraDatacenter Instances",
		"columns.instance": "CassandraDatacenter Instance",
		"columns.namespace": "Namespace",
		"columns.health": "Health",
	}
}

func buildCatalog() Catalog {
	catalog := map[string]string{}
	return utils.MergeMap(
		catalog,
		prefixKeys(buildInstanceHealthCatalog(), "CassandraDatacenter.instaceHealth."),
	)
}

func marshalMapToProperties(m map[string]string) string {
	var b strings.Builder
	for k, v := range m {
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	return b.String()
}

func createCatalogConfigMap(catalog Catalog, catalogName types.NamespacedName) (*corev1.ConfigMap) {
	content := marshalMapToProperties(catalog)
	configMap := newConfigMap(catalogName)
	configMap.Data["health_en"] = content
	utils.AddHashAnnotation(configMap)
	configMap.SetLabels(
		utils.MergeMap(
			map[string]string{}, 
			configMap.GetLabels(), 
			map[string]string{HealthLabel: HealthLabelCatalogValue}))

	return configMap
}

func newConfigMap(name types.NamespacedName) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{}
	configMap.ObjectMeta.Name = name.Name
	configMap.ObjectMeta.Namespace = name.Namespace
	configMap.BinaryData = map[string][]byte{}
	configMap.Data = map[string]string{}

	return configMap
}

func getConfigMap(client runtimeClient.Client, name types.NamespacedName) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{
	}
	err := client.Get(
		context.TODO(),
		name,
		configMap)

	if err != nil && errors.IsNotFound(err) {
		return nil, nil
	}
	return configMap, err
}

func upsertConfigMap(client runtimeClient.Client, configMap *corev1.ConfigMap) error {
	name := types.NamespacedName{Name: configMap.GetName(), Namespace: configMap.GetNamespace()}
	existingConfigMap, err := getConfigMap(client, name)
	if err != nil {
		return err
	}

	if existingConfigMap == nil {
		// Create new config map
		return client.Create(context.TODO(), configMap)
	} else if !utils.ResourcesHaveSameHash(configMap, existingConfigMap) {
		// Update the existing config map
		resourceVersion := existingConfigMap.GetResourceVersion()

		configMap.Labels = utils.MergeMap(
			map[string]string{}, 
			existingConfigMap.Labels, 
			configMap.Labels)

		configMap.Annotations = utils.MergeMap(
			map[string]string{}, 
			existingConfigMap.Annotations, 
			configMap.Annotations)

		configMap.DeepCopyInto(existingConfigMap)
		existingConfigMap.SetResourceVersion(resourceVersion)

		return client.Update(context.TODO(), existingConfigMap)
	}

	// No update needed
	return nil
}
