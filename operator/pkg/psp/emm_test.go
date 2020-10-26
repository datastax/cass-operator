// Copyright DataStax, Inc.
// Please see the included license file for details.

package psp

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/go-logr/logr"
	logrtesting "github.com/go-logr/logr/testing"
	

	"github.com/datastax/cass-operator/operator/internal/result"
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

func (m *MockEMMService) getNodeNameSetForRack(rackName string) map[string]bool {
	args := m.Called(rackName)
	return args.Get(0).(map[string]bool)
}

func (m *MockEMMService) cleanupEMMAnnotations() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) getNodeNameSetForPlannedDownTime() (map[string]bool, error) {
	args := m.Called()
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockEMMService) getNodeNameSetForEvacuateAllData() (map[string]bool, error) {
	args := m.Called()
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockEMMService) removeAllNotReadyPodsOnEMMNodes() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) failEMM(nodeName string, failure EMMFailure) (bool, error) {
	args := m.Called(nodeName, failure)
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) performPodReplaceForEvacuateData() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) removeNextPodFromEvacuateDataNode() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockEMMService) removeAllPodsFromPlannedDowntimeNode() (bool, error) {
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

func (m *MockEMMService) getSelectedNodeNameForPodPVC(podName string) (string, error) {
	args := m.Called(podName)
	return args.String(0), args.Error(1)
}

func (m *MockEMMService) getPodNameSetWithVolumeHealthInaccessiblePVC(rackName string) (map[string]bool, error) {
	args := m.Called(rackName)
	return args.Get(0).(map[string]bool), args.Error(1)
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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{"node1": true, "node2": true}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node3": true}, nil)
	testObj.On("failEMM", "node3", GenericFailure).Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")	


	// When there are no extraneous EMM failure annotations, and there are 
	// pods not ready on multiple nodes, fail EMM for all nodes tainted for
	// EMM
	testObj = &MockEMMService{}
	testObj.On("cleanupEMMAnnotations").Return(false, nil)
	testObj.On("IsInitialized").Return(true)
	testObj.On("IsStopped").Return(false)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1", "rack2"})
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{"node1": true, "node2": true}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node3": true}, nil)
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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{"node1": true, "node2": true}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node3": true}, nil)
	testObj.On("getNodeNameSetForRack", "rack1").Return(map[string]bool{"node2": true})
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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{"node1": true, "node2": true}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{}, nil)
	testObj.On("getNodeNameSetForRack", "rack1").Return(map[string]bool{"node1": true, "node2": true})
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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node2": true}, nil)
	testObj.On("getNodeNameSetForRack", "rack1").Return(map[string]bool{"node1": true, "node2": true})
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("performPodReplaceForEvacuateData").Return(true, nil)

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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node2": true}, nil)
	testObj.On("getNodeNameSetForRack", "rack1").Return(map[string]bool{"node1": true, "node2": true})
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("performPodReplaceForEvacuateData").Return(false, nil)

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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node2": true}, nil)
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
	testObj.On("getNodeNameSetForPlannedDownTime").Return(map[string]bool{}, nil)
	testObj.On("getNodeNameSetForEvacuateAllData").Return(map[string]bool{"node2": true}, nil)
	testObj.On("removeAllNotReadyPodsOnEMMNodes").Return(false, nil)
	testObj.On("removeNextPodFromEvacuateDataNode").Return(false, nil)
	testObj.On("removeAllPodsFromPlannedDowntimeNode").Return(true, nil)

	r = checkNodeEMM(testObj)
	testObj.AssertExpectations(t)
	IsRequeue(t, r, "")
}


func Test_checkPVCHealth(t *testing.T) {
	// When no pods have PVCs marked as inaccessible, do nothing and continue
	// reconciliation
	testObj := &MockEMMService{}
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "").Return(map[string]bool{}, nil)

	r := checkPVCHealth(testObj)
	IsContinue(t, r, "")

	// When pods not ready on more than one rack, do nothing and continue
	testObj = &MockEMMService{}
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "").Return(map[string]bool{}, nil)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1", "rack2"})

	r = checkPVCHealth(testObj)
	IsContinue(t, r, "")

	// When one rack has not ready pods, limit pods considered for replacement
	// to those on the impacted rack
	testObj = &MockEMMService{}
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "").Return(map[string]bool{"pod-1": true}, nil)
	testObj.On("getRacksWithNotReadyPodsBootstrapped").Return([]string{"rack1"})
	testObj.On("getPodNameSetWithVolumeHealthInaccessiblePVC", "rack1").Return(map[string]bool{"pod-1": true}, nil)
	testObj.On("getInProgressNodeReplacements").Return([]string{})
	testObj.On("startNodeReplace", "pod-1").Return(nil)

	r = checkPVCHealth(testObj)
	IsRequeue(t, r, "")
}
