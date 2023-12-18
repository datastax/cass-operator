module github.com/datastax/cass-operator

go 1.14

require (
	github.com/Jeffail/gabs v1.4.0
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.4
	github.com/google/uuid v1.1.1
	github.com/magefile/mage v1.9.0
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/pavel-v-chernykh/keystore-go v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	golang.org/x/crypto v0.17.0
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.17.4
	k8s.io/gengo v0.0.0-20191010091904-7fa3014cb28f
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kubernetes v1.17.4
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/controller-tools v0.2.8
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/api => k8s.io/api v0.17.4
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.17.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.4
	k8s.io/apiserver => k8s.io/apiserver v0.17.4
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.17.4
	k8s.io/client-go => k8s.io/client-go v0.17.4
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.17.4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.17.4
	k8s.io/code-generator => k8s.io/code-generator v0.17.4
	k8s.io/component-base => k8s.io/component-base v0.17.4
	k8s.io/cri-api => k8s.io/cri-api v0.17.4
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.17.4
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.17.4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.17.4
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.17.4
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.17.4
	k8s.io/kubectl => k8s.io/kubectl v0.17.4
	k8s.io/kubelet => k8s.io/kubelet v0.17.4
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.17.4
	k8s.io/metrics => k8s.io/metrics v0.17.4
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.17.4
)
