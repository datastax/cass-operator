// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

//
// This file defines helpers for unit testing.
//

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/datastax/cass-operator/operator/pkg/psp"
	"github.com/go-logr/logr"
	mock "github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	log2 "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
	"github.com/datastax/cass-operator/operator/pkg/httphelper"
	"github.com/datastax/cass-operator/operator/pkg/mocks"
)

// MockSetControllerReference returns a method that will automatically reverse the mock
func MockSetControllerReference() func() {
	oldSetControllerReference := setControllerReference
	setControllerReference = func(
		owner,
		object metav1.Object,
		scheme *runtime.Scheme) error {
		return nil
	}

	return func() {
		setControllerReference = oldSetControllerReference
	}
}

// CreateMockReconciliationContext ...
func CreateMockReconciliationContext(
	reqLogger logr.Logger) *ReconciliationContext {

	// These defaults may need to be settable via arguments

	var (
		name              = "cassandradatacenter-example"
		clusterName       = "cassandradatacenter-example-cluster"
		namespace         = "default"
		size        int32 = 2
	)

	storageSize := resource.MustParse("1Gi")
	storageName := "server-data"
	storageConfig := api.StorageConfig{
		CassandraDataVolumeClaimSpec: &corev1.PersistentVolumeClaimSpec{
			StorageClassName: &storageName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{"storage": storageSize},
			},
		},
	}

	// Instance a cassandraDatacenter
	cassandraDatacenter := &api.CassandraDatacenter{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: api.CassandraDatacenterSpec{
			Size:          size,
			ClusterName:   clusterName,
			ServerType:    "dse",
			ServerVersion: "6.8.3",
			StorageConfig: storageConfig,
		},
	}

	// Objects to keep track of

	trackObjects := []runtime.Object{
		cassandraDatacenter,
	}

	s := scheme.Scheme
	s.AddKnownTypes(api.SchemeGroupVersion, cassandraDatacenter)

	fakeClient := fake.NewFakeClient(trackObjects...)

	request := &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	rc := &ReconciliationContext{}
	rc.Request = request
	rc.Client = fakeClient
	rc.Scheme = s
	rc.ReqLogger = reqLogger
	rc.Datacenter = cassandraDatacenter
	rc.Recorder = record.NewFakeRecorder(100)
	rc.Ctx = context.Background()

	res := &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader("OK")),
	}

	mockHttpClient := &mocks.HttpClient{}
	mockHttpClient.On("Do",
		mock.MatchedBy(
			func(req *http.Request) bool {
				return req != nil
			})).
		Return(res, nil)

	rc.NodeMgmtClient = httphelper.NodeMgmtClient{Client: mockHttpClient, Log: reqLogger, Protocol: "http"}

	rc.PSPHealthUpdater = &psp.NoOpUpdater{}

	return rc
}

// Create a fake client that is tracking a service
func fakeClientWithService(
	cassandraDatacenter *api.CassandraDatacenter) (*client.Client, *corev1.Service) {

	service := newServiceForCassandraDatacenter(cassandraDatacenter)

	// Objects to keep track of

	trackObjects := []runtime.Object{
		cassandraDatacenter,
		service,
	}

	fakeClient := fake.NewFakeClient(trackObjects...)

	return &fakeClient, service
}

func setupTest() (*ReconciliationContext, *corev1.Service, func()) {
	// Set up verbose logging
	logger := zap.Logger(true)
	log2.SetLogger(logger)
	cleanupMockScr := MockSetControllerReference()

	rc := CreateMockReconciliationContext(logger)
	service := newServiceForCassandraDatacenter(rc.Datacenter)

	return rc, service, cleanupMockScr
}

func k8sMockClientGet(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Get",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(key client.ObjectKey) bool {
				return key != client.ObjectKey{}
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientUpdate(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Update",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientCreate(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Create",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientDelete(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("Delete",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			})).
		Return(returnArg).
		Once()
}

func k8sMockClientList(mockClient *mocks.Client, returnArg interface{}) *mock.Call {
	return mockClient.On("List",
		mock.MatchedBy(
			func(ctx context.Context) bool {
				return ctx != nil
			}),
		mock.MatchedBy(
			func(obj runtime.Object) bool {
				return obj != nil
			}),
		mock.MatchedBy(
			func(opts *client.ListOptions) bool {
				return opts != nil
			})).
		Return(returnArg).
		Once()
}
