// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

// This file defines constructors for k8s service-related objects
import (
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Creates a headless service object for the Datacenter, for clients wanting to
// reach out to a ready Server node for either CQL or mgmt API
func newServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	svcName := dc.GetDatacenterServiceName()
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = svcName

	nativePort := api.DefaultNativePort
	if dc.IsNodePortEnabled() {
		nativePort = dc.GetNodePortNativePort()
	}

	ports := []corev1.ServicePort{
		namedServicePort("native", nativePort, nativePort),
		namedServicePort("tls-native", 9142, 9142),
		namedServicePort("mgmt-api", 8080, 8080),
		namedServicePort("prometheus", 9103, 9103),
		namedServicePort("thrift", 9160, 9160),
	}

	if dc.Spec.DseWorkloads != nil {
		if dc.Spec.DseWorkloads.AnalyticsEnabled {
			ports = append(
				ports,
				namedServicePort("dsefs-public", 5598, 5598),
				namedServicePort("spark-worker", 7081, 7081),
				namedServicePort("jobserver", 8090, 8090),
				namedServicePort("always-on-sql", 9077, 9077),
				namedServicePort("sql-thrift", 10000, 10000),
				namedServicePort("spark-history", 18080, 18080),
			)
		}

		if dc.Spec.DseWorkloads.GraphEnabled {
			ports = append(
				ports,
				namedServicePort("gremlin", 8182, 8182),
			)
		}

		if dc.Spec.DseWorkloads.SearchEnabled {
			ports = append(
				ports,
				namedServicePort("solr", 8983, 8983),
			)
		}
	}

	service.Spec.Ports = ports

	utils.AddHashAnnotation(service)

	return service
}

func namedServicePort(name string, port int, targetPort int) corev1.ServicePort {
	return corev1.ServicePort{Name: name, Port: int32(port), TargetPort: intstr.FromInt(targetPort)}
}

func buildLabelSelectorForSeedService(dc *api.CassandraDatacenter) map[string]string {
	labels := dc.GetClusterLabels()

	// narrow selection to just the seed nodes
	labels[api.SeedNodeLabel] = "true"

	return labels
}

// newSeedServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter which will attach to all seed
// nodes in the cluster
func newSeedServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetSeedServiceName()

	labels := dc.GetClusterLabels()
	oplabels.AddManagedByLabel(labels)
	service.ObjectMeta.Labels = labels

	service.Spec.Selector = buildLabelSelectorForSeedService(dc)
	service.Spec.PublishNotReadyAddresses = true

	utils.AddHashAnnotation(service)

	return service
}

// newAdditionalSeedServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// whether the additional seed pods are ready or not
func newAdditionalSeedServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	var service corev1.Service
	service.ObjectMeta.Name = dc.GetAdditionalSeedsServiceName()
	service.ObjectMeta.Namespace = dc.Namespace
	service.ObjectMeta.Labels = labels
	// We omit the label selector because we will create the endpoints manually
	service.Spec.Type = "ClusterIP"
	service.Spec.ClusterIP = "None"
	service.Spec.PublishNotReadyAddresses = true

	utils.AddHashAnnotation(&service)

	return &service
}

func newEndpointsForAdditionalSeeds(dc *api.CassandraDatacenter) *corev1.Endpoints {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	var endpoints corev1.Endpoints
	endpoints.ObjectMeta.Name = dc.GetAdditionalSeedsServiceName()
	endpoints.ObjectMeta.Namespace = dc.Namespace
	endpoints.ObjectMeta.Labels = labels

	var addresses []corev1.EndpointAddress

	for seedIdx := range dc.Spec.AdditionalSeeds {
		additionalSeed := dc.Spec.AdditionalSeeds[seedIdx]
		addresses = append(addresses, corev1.EndpointAddress{
			IP: additionalSeed,
		})
	}

	// See: https://godoc.org/k8s.io/api/core/v1#Endpoints
	endpoints.Subsets = []corev1.EndpointSubset{
		{
			Addresses: addresses,
		},
	}

	utils.AddHashAnnotation(&endpoints)

	return &endpoints
}

// newNodePortServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// that preserves the client source IPs
func newNodePortServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetNodePortServiceName()

	service.Spec.Type = "NodePort"
	// Note: ClusterIp = "None" is not valid for NodePort
	service.Spec.ClusterIP = ""
	service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeLocal

	nativePort := dc.GetNodePortNativePort()
	internodePort := dc.GetNodePortInternodePort()

	service.Spec.Ports = []corev1.ServicePort{
		// Note: Port Names cannot be more than 15 characters
		{
			Name:       "internode",
			Port:       int32(internodePort),
			NodePort:   int32(internodePort),
			TargetPort: intstr.FromInt(internodePort),
		},
		{
			Name:       "native",
			Port:       int32(nativePort),
			NodePort:   int32(nativePort),
			TargetPort: intstr.FromInt(nativePort),
		},
	}

	return service
}

// newAllPodsServiceForCassandraDatacenter creates a headless service owned by the CassandraDatacenter,
// which covers all server pods in the datacenter, whether they are ready or not
func newAllPodsServiceForCassandraDatacenter(dc *api.CassandraDatacenter) *corev1.Service {
	service := makeGenericHeadlessService(dc)
	service.ObjectMeta.Name = dc.GetAllPodsServiceName()
	service.Spec.PublishNotReadyAddresses = true

	nativePort := api.DefaultNativePort
	if dc.IsNodePortEnabled() {
		nativePort = dc.GetNodePortNativePort()
	}

	service.Spec.Ports = []corev1.ServicePort{
		{
			Name: "native", Port: int32(nativePort), TargetPort: intstr.FromInt(nativePort),
		},
		{
			Name: "mgmt-api", Port: 8080, TargetPort: intstr.FromInt(8080),
		},
		{
			Name: "prometheus", Port: 9103, TargetPort: intstr.FromInt(9103),
		},
	}

	utils.AddHashAnnotation(service)

	return service
}

// makeGenericHeadlessService returns a fresh k8s headless (aka ClusterIP equals "None") Service
// struct that has the same namespace as the CassandraDatacenter argument, and proper labels for the DC.
// The caller needs to fill in the ObjectMeta.Name value, at a minimum, before it can be created
// inside the k8s cluster.
func makeGenericHeadlessService(dc *api.CassandraDatacenter) *corev1.Service {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	selector := dc.GetDatacenterLabels()
	var service corev1.Service
	service.ObjectMeta.Namespace = dc.Namespace
	service.ObjectMeta.Labels = labels
	service.Spec.Selector = selector
	service.Spec.Type = "ClusterIP"
	service.Spec.ClusterIP = "None"
	return &service
}
