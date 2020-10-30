// Copyright DataStax, Inc.
// Please see the included license file for details.

package psp

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	logrtesting "github.com/go-logr/logr/testing"
	

	"github.com/datastax/cass-operator/operator/internal/result"
	"github.com/datastax/cass-operator/operator/pkg/utils"
)

func IsRequeue(t *testing.T, r result.ReconcileResult, msg string) {
	isRequeue := false
	if r.Completed() == true {
		v, err := r.Output()
		if err == nil {
			isRequeue = v.Requeue
		}
	}
	require.True(t, isRequeue, msg)
}

func IsContinue(t *testing.T, r result.ReconcileResult, msg string) {
	require.False(t, r.Completed(), msg)
}

type MockEMMService struct {
	mock.Mock
}

func (m *MockEMMService) getRacksWithNotReadyPodsBootstrapped() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockEMMService) getRackNodeNameSet(rackName string) utils.StringSet {
	args := m.Called(rackName)
	return args.Get(0).(utils.StringSet)
}

func (m *MockEMMService) cleanupEMMAnnotations() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) getPlannedDownTimeNodeNameSet() (utils.StringSet, error) {
	args := m.Called()
	return args.Get(0).(utils.StringSet), args.Error(1)
}

func (m *MockEMMService) getEvacuateAllDataNodeNameSet() (utils.StringSet, error) {
	args := m.Called()
	return args.Get(0).(utils.StringSet), args.Error(1)
}

func (m *MockEMMService) removeAllNotReadyPodsOnEMMNodes() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) failEMM(nodeName string, failure EMMFailure) (bool, error) {
	args := m.Called(nodeName, failure)
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) performEvacuateDataPodReplace() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) removeNextPodFromEvacuateDataNode() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) removeAllPodsFromOnePlannedDowntimeNode() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) IsStopped() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockEMMService) IsInitialized() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockEMMService) startNodeReplace(podName string) error {
	args := m.Called(podName)
	return args.Error(0)
}

func (m *MockEMMService) getInProgressNodeReplacements() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockEMMService) getPodPVCSelectedNodeName(podName string) (string, error) {
	args := m.Called(podName)
	return args.String(0), args.Error(1)
}

func (m *MockEMMService) getPodNameSetWithVolumeHealthInaccessiblePVC(rackName string) (utils.StringSet, error) {
	args := m.Called(rackName)
	return args.Get(0).(utils.StringSet), args.Error(1)
}

func (m *MockEMMService) getLogger() logr.Logger {
	return logrtesting.NullLogger{}
}

func Test_checkNodeEMM(t *testing.T) {
	// When there are pods annotated with EMM failure that are not on an EMM
	// tainted node, then the annotations should be cleared
	testObj := &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(true, nil)

	r := checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")

	
	// When datacenter is not initialized, ignore EMM taints
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(false)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsContinue(t, r, "")


	// When datacenter is not stopped, fail all EMM operations to evacuate 
	// data as cassandra is not running to rebuild the impacted cassandra
	// nodes
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(true)
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{"node1": true, "node2": true}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node3": true}, nil)
	testObj.On("failEMM", "node3", GenericFailure).Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")	


	// When there are no extraneous EMM failure annotations, and there are 
	// pods not ready on multiple racks, fail EMM for all nodes tainted for
	// EMM
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1", "rack2"})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{"node1": true, "node2": true}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node3": true}, nil)
	testObj.On("failEMM", "node1", TooManyExistingFailures).Return(true, nil)
	testObj.On("failEMM", "node2", TooManyExistingFailures).Return(true, nil)
	testObj.On("failEMM", "node3", TooManyExistingFailures).Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")


	// When there are no extraneous EMM failure annotations, and there are 
	// pods not ready on one rack, fail EMM for all nodes not part of that 
	// rack
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1"})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{"node1": true, "node2": true}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node3": true}, nil)
	testObj.On("getRackNodeNameSet", "rack1").Return(utils.StringSet{"node2": true})
	testObj.On("failEMM", "node1", TooManyExistingFailures).Return(true, nil)
	testObj.On("failEMM", "node3", TooManyExistingFailures).Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")


	// When there are no extraneous EMM failure annotations, there are not
	// multiple racks with pods not ready, and all nodes marked for EMM have pods
	// for the rack with not ready pods, delete any not ready pods on the nodes
	// marked for EMM.
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1"})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{"node1": true, "node2": true}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{}, nil)
	testObj.On("getRackNodeNameSet", "rack1").Return(utils.StringSet{"node1": true, "node2": true})
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")

	// When there are no extraneous EMM failure annotations, therea are not
	// multiple racks with pods not ready, and all nodes marked for EMM have 
	// pods for the rack with not ready pods, there are no not ready pods on
	// the nodes marked for EMM, perform a replace on any pod with a PVC
	// associated with a node marked for EMM evacuate data.
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1"})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node2": true}, nil)
	testObj.On("getRackNodeNameSet", "rack1").Return(utils.StringSet{"node1": true, "node2": true})
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("performEvacuateDataPodReplace").Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")


	// When there are no extraneous EMM failure annotations, therea are not
	// multiple racks with pods not ready, and all nodes marked for EMM have 
	// pods for the rack with not ready pods, there are no not ready pods on
	// the nodes marked for EMM, and there are pods not ready not on the EMM
	// nodes that are scheduable, return and continue to allow the pods to
	// start and--hopefully--become ready.
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1"})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node2": true}, nil)
	testObj.On("getRackNodeNameSet", "rack1").Return(utils.StringSet{"node1": true, "node2": true})
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("performEvacuateDataPodReplace").Return(false, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsContinue(t, r, "should continue reconcile to allow cassandra to be started on not ready pods")


	// When there are no pods not ready, remove a pod from an EMM node
	// marked for evacuate all data.
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node2": true}, nil)
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("removeNextPodFromEvacuateDataNode").Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")

	
	// When there are no pods not ready and no nodes are marked for evacuate
	// data, remove all pods for a node marked for planned down time
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{})
	testObj.On("getPlannedDownTimeNodeNameSet").Return(utils.StringSet{}, nil)
	testObj.On("getEvacuateAllDataNodeNameSet").Return(utils.StringSet{"node2": true}, nil)
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("removeNextPodFromEvacuateDataNode").Return(false, nil)
	testObj.On("removeAllPodsFromOnePlannedDowntimeNode").Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")
}


func Test_checkPVCHealth(t *testing.T) {
	// When no pods have PVCs marked as inaccessible, do nothing and continue
	// reconciliation
	testObj := &MockEMMService{}
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "").Return(utils.StringSet{}, nil)

	r := checkPVCHealth(testObj)
	IsContinue(t, r, "")

	// When pods not ready on more than one rack, do nothing and continue
	testObj = &MockEMMService{}
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "").Return(utils.StringSet{}, nil)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1", "rack2"})

	r = checkPVCHealth(testObj)
	IsContinue(t, r, "")

	// When one rack has not ready pods, limit pods considered for replacement
	// to those on the impacted rack
	testObj = &MockEMMService{}
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "").Return(utils.StringSet{"pod-1": true}, nil)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1"})
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "rack1").Return(utils.StringSet{"pod-1": true}, nil)
	testObj.On("getInProgressNodeReplacements").Return([]string{})
	testObj.On("startNodeReplace", "pod-1").Return(nil)

	r = checkPVCHealth(testObj)
	IsRequeue(t, r, "")
}

type MockEMMSPI struct {
	mock.Mock
}

func (m *MockEMMSPI) GetAllNodesInDC() ([]*corev1.Node, error) {
	args := m.Called()
	return args.Get(0).([]*corev1.Node), args.Error(1)
}

func (m *MockEMMSPI) GetDCPods() []*corev1.Pod {
	args := m.Called()
	return args.Get(0).([]*corev1.Pod)
}

func (m *MockEMMSPI) GetNotReadyPodsBootstrappedInDC() []*corev1.Pod {
	args := m.Called()
	return args.Get(0).([]*corev1.Pod)
}

func (m *MockEMMSPI) GetAllPodsNotReadyInDC() []*corev1.Pod {
	args := m.Called()
	return args.Get(0).([]*corev1.Pod)
}

func (m *MockEMMSPI) GetPodPVCs(pod *corev1.Pod) ([]*corev1.PersistentVolumeClaim, error) {
	args := m.Called(pod)
	return args.Get(0).([]*corev1.PersistentVolumeClaim), args.Error(1)
}

func (m *MockEMMSPI) StartNodeReplace(podName string) error {
	args := m.Called(podName)
	return args.Error(0)
}

func (m *MockEMMSPI) GetInProgressNodeReplacements() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockEMMSPI) RemovePod(pod *corev1.Pod) error {
	args := m.Called(pod)
	return args.Error(0)
}

func (m *MockEMMSPI) UpdatePod(pod *corev1.Pod) error {
	args := m.Called(pod)
	return args.Error(0)
}

func (m *MockEMMSPI) IsStopped() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockEMMSPI) IsInitialized() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockEMMSPI) GetLogger() logr.Logger {
	return logrtesting.NullLogger{}
}

func pod(name string, nodeName string) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Name = name
	pod.Spec.NodeName = nodeName

	return pod
}

func evacuateDataNode(name string) *corev1.Node {
	node := &corev1.Node{}
	node.Name = name
	node.Spec.Taints = []corev1.Taint{
		corev1.Taint{
			Key: "node.vmware.com/drain",
			Value: "drain",
			Effect: corev1.TaintEffectNoSchedule,
		},
	}
	return node
}

func Test_removeAllNotReadyPodsOnEMMNodes(t *testing.T) {
	var service *EMMServiceImpl
	var testObj *MockEMMSPI

	testObj = &MockEMMSPI{}
	service = &EMMServiceImpl {
		EMMSPI: testObj,
	}

	pod := pod("pod1", "node1")
	testObj.On("GetAllPodsNotReadyInDC").Return([]*corev1.Pod{pod})
	testObj.On("GetAllNodesInDC").Return([]*corev1.Node{evacuateDataNode("node1")}, nil)
	testObj.On("RemovePod", pod).Return(nil)

	changed, err := service.removeAllNotReadyPodsOnEMMNodes()
	testObj.AssertExpectations(t)
	require.True(t, changed, "should have removed a not ready pod")
	require.Nil(t, err, "should not have encountered an error")
}
