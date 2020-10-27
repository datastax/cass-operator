package psp

import (
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

const (
	ExtensionIDLabel string        = "appplatform.vmware.com/extension-id"
	InstanceIDLabel string         = "appplatform.vmware.com/instance-id"
	EMMIntegratedAnnotation string = "appplatform.vmware.com/vsphere-emm-integrated"
	ExtensionIDEnv string          = "PSP_EXTENSION_ID"
)

func GetExtensionID() string {
	value := os.Getenv(ExtensionIDEnv)
	if value == "" {
		value = "datastax-vulcan"
	}
	return value
}

func AddStatefulSetChanges(dc *api.CassandraDatacenter, statefulSet *appsv1.StatefulSet) *appsv1.StatefulSet {
	for i, _ := range statefulSet.Spec.VolumeClaimTemplates {
		cvt := &statefulSet.Spec.VolumeClaimTemplates[i]
		addLabels(dc.Name, cvt)
	}
	
	podTemplate := &statefulSet.Spec.Template
	addLabels(dc.Name, podTemplate)
	addAnnotations(podTemplate)

	return statefulSet
}

func addAnnotations(obj metav1.Object) {
	annos := obj.GetAnnotations()
	if annos == nil {
		annos = map[string]string{}
	}
	annos[EMMIntegratedAnnotation] = "true"
	obj.SetAnnotations(annos)
}

func addLabels(dcName string, obj metav1.Object) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}

	labels[ExtensionIDLabel] = GetExtensionID()
	labels[InstanceIDLabel] = dcName

	obj.SetLabels(labels)
}
