package psp

import (
	"fmt"
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkingv1 "k8s.io/api/networking/v1"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/oplabels"
	"github.com/datastax/cass-operator/operator/internal/result"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

type CheckNetworkPoliciesSPI interface {
	GetClient() client.Client
	GetLogger() logr.Logger
	GetContext() context.Context
	GetDatacenter() *api.CassandraDatacenter
	SetDatacenterAsOwner(controlled metav1.Object) error
}

func newNetworkPolicyForCassandraDatacenter(dc *api.CassandraDatacenter) *networkingv1.NetworkPolicy {
	labels := dc.GetDatacenterLabels()
	oplabels.AddManagedByLabel(labels)
	selector := dc.GetDatacenterLabels()

	ingressRule := networkingv1.NetworkPolicyIngressRule{}

	policy := &networkingv1.NetworkPolicy{}
	policy.ObjectMeta.Name = fmt.Sprintf("%s-management-api-ingress", dc.Name)
	policy.ObjectMeta.Namespace = dc.Namespace
	policy.ObjectMeta.Labels = labels
	policy.Spec.PodSelector.MatchLabels = selector
	policy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}
	policy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{ingressRule}

	utils.AddHashAnnotation(policy)
	return policy
}

// VMWare with Kubernetes does not permit network traffic between namespaces
// by default. This means a NetworkPolicy must be created to allow the 
// operator to make requests to the Management API.
//
// NOTE: The way VMWare implements NetworkPolicy appears to be non-standard.
// For example, typially setting the namespace selector to an empty value
// _should_ select all namespaces, but it does not in VMWare with Kubernetes.
// Consequently, it is important that changes here be verified in that 
// environment.
func CheckNetworkPolicies(spi CheckNetworkPoliciesSPI) result.ReconcileResult {
	logger := spi.GetLogger()
	dc := spi.GetDatacenter()
	client := spi.GetClient()
	ctx := spi.GetContext()

	logger.Info("psp::CheckNetworkPolicies")

	desiredPolicy := newNetworkPolicyForCassandraDatacenter(dc)

	// Set CassandraDatacenter dc as the owner and controller
	err := spi.SetDatacenterAsOwner(desiredPolicy)
	if err != nil {
		logger.Error(err, "Could not set controller reference for network policy")
		return result.Error(err)
	}

	// See if the service already exists
	nsName := types.NamespacedName{Name: desiredPolicy.Name, Namespace: desiredPolicy.Namespace}
	currentPolicy := &networkingv1.NetworkPolicy{}
	err = client.Get(ctx, nsName, currentPolicy)

	if err != nil && errors.IsNotFound(err) {
		if err := client.Create(ctx, desiredPolicy); err != nil {
			logger.Error(err, "Could not create network policy")

			return result.Error(err)
		}
	} else if err != nil {
		// if we hit a k8s error, log it and error out
		logger.Error(err, "Could not get network policy",
			"name", nsName,
		)
		return result.Error(err)

	} else {
		// if we found the service already, check if they need updating
		if !utils.ResourcesHaveSameHash(currentPolicy, desiredPolicy) {
			resourceVersion := currentPolicy.GetResourceVersion()
			// preserve any labels and annotations that were added to the service post-creation
			desiredPolicy.Labels = utils.MergeMap(map[string]string{}, currentPolicy.Labels, desiredPolicy.Labels)
			desiredPolicy.Annotations = utils.MergeMap(map[string]string{}, currentPolicy.Annotations, desiredPolicy.Annotations)

			logger.Info("Updating network policy",
				"policy", currentPolicy,
				"desired", desiredPolicy)

			desiredPolicy.DeepCopyInto(currentPolicy)

			currentPolicy.SetResourceVersion(resourceVersion)

			if err := client.Update(ctx, currentPolicy); err != nil {
				logger.Error(err, "Unable to update network policy",
					"policy", currentPolicy)
				return result.Error(err)
			}
		}
	}
	
	return result.Continue()
}
