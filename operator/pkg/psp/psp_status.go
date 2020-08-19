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

func buildCatalog() map[string]string {
	catalog := map[string]string{}
	return utils.MergeMap(
		catalog,
		prefixKeys(buildInstanceHealthCatalog(), "CassandraDatacenter.instaceHealth."),
	)
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

type DAO interface {
	GetHealthCheckConfigMap() (*corev1.ConfigMap, error)
	UpsertHealthCheckConfigMap(configMap *corev1.ConfigMap) error
	UpsertCatalogConfigMap(configMap *corev1.ConfigMap) error
}

type DAOImpl struct {
	catalogNamespacedName types.NamespacedName
	healthChecksNamespacedName types.NamespacedName
	client runtimeClient.Client
}

func getConfigMap(client runtimeClient.Client, name types.NamespacedName) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{

	}
	err := client.Get(
		context.TODO(),
		name,
		configMap)

	if err != nil && errors.IsNotFound(err) {
		configMap.ObjectMeta.Name = name.Name
		configMap.ObjectMeta.Namespace = name.Namespace
		return configMap, nil
	} else {
		return configMap, err
	}
}

func (dao *DAOImpl) GetHealthCheckConfigMap() (*corev1.ConfigMap, error) {
	return getConfigMap(dao.client, dao.healthChecksNamespacedName)
}

func (dao *DAOImpl) UpsertHealthCheckConfigMap(configMap *corev1.ConfigMap) error {
	return dao.client.Update(context.TODO(), configMap)
}

func (dao *DAOImpl) UpsertCatalogConfigMap(configMap *corev1.ConfigMap) error {
	return dao.client.Update(context.TODO(), configMap)
}

type HealthStatusUpdaterImpl struct {
	dao DAO
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

func New(client runtimeClient.Client, namespace string) HealthStatusUpdater {
	return &HealthStatusUpdaterImpl{
		dao: NewDao(client, namespace),
	}
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

func saveHealthCheckToConfigMap(health *Health, configMap *corev1.ConfigMap) error {
	healthRaw, err := yaml.Marshal(health)
	if err != nil {
		return err
	}

	configMap.BinaryData["health"] = healthRaw

	// TODO populate misc. other fields

	return nil
}

func (updater *HealthStatusUpdaterImpl) Update(dc api.CassandraDatacenter) error {
	dao := updater.dao
	healthCheckMap, err := dao.GetHealthCheckConfigMap()
	if err != nil {
		return err
	}

	health, err := loadHealthFromConfigMap(healthCheckMap)
	if err != nil {
		return err
	}

	updateHealth(health, dc)

	err = saveHealthCheckToConfigMap(health, healthCheckMap)
	if err != nil {
		return err
	}

	return dao.UpsertHealthCheckConfigMap(healthCheckMap)
}
