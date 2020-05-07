module github.com/datastax/cass-operator

go 1.13

require (
	bitbucket.org/ww/goautoneg v0.0.0-20120707110453-75cd24fc2f2c // indirect
	github.com/Jeffail/gabs v1.4.0
	github.com/Masterminds/sprig v2.22.0+incompatible // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.0 // indirect
	github.com/emicklei/go-restful v2.9.3+incompatible // indirect
	github.com/emicklei/go-restful-swagger12 v0.0.0-20170926063155-7524189396c6 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.4
	github.com/golang/lint v0.0.0-20180702182130-06c8688daad7 // indirect
	github.com/google/uuid v1.1.1
	github.com/gregjones/httpcache v0.0.0-20190203031600-7a902570cb17 // indirect
	github.com/lib/pq v1.2.0 // indirect
	github.com/magefile/mage v1.9.0
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/maxbrunsfeld/counterfeiter v0.0.0-20181017030959-1aadac120687 // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.8.1
	github.com/openshift/api v3.9.1-0.20190424152011-77b8897ec79a+incompatible // indirect
	github.com/operator-framework/go-appr v0.0.0-20180917210448-f2aef88446f2 // indirect
	github.com/operator-framework/operator-marketplace v0.0.0-20190216021216-57300a3ef3ba // indirect
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c // indirect
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	github.com/stevvooe/resumable v0.0.0-20180830230917-22b14a53ba50 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/technosophos/moniker v0.0.0-20180509230615-a5dbd03a2245 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.17.4
	k8s.io/gengo v0.0.0-20191010091904-7fa3014cb28f
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	k8s.io/kubernetes v1.15.4
	sigs.k8s.io/controller-runtime v0.5.2
	sigs.k8s.io/controller-tools v0.2.8
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
)

// Pinned to kubernetes-1.15.4
replace (
	k8s.io/api => k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190918201827-3de75813f604
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20190918200908-1e17798da8c1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20190918202139-0b14c719ca62
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20190918203125-ae665f80358a
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190918202959-c340507a5d48
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190612205613-18da4a14b22b
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190918200425-ed2f0867c778
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190817025403-3ae76f584e79
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20190918203248-97c07dcbb623
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20190918201136-c3a845f1fbb2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20190918202837-c54ce30c680e
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20190918202429-08c8357f8e2d
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20190918202713-c34a54b3ec8e
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20190918202550-958285cf3eef
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20190918203421-225f0541b3ea
	k8s.io/metrics => k8s.io/metrics v0.0.0-20190918202012-3c1ca76f5bda
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20190918201353-5cc279503896
)
