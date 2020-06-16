package events

import (
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// Events
	UpdatingRack                      string = "UpdatingRack"
	StoppingDatacenter                string = "StoppingDatacenter"
	DeletingStuckPod                  string = "DeletingStuckPod"
	RestartingCassandra               string = "RestartingCassandra"
	CreatedResource                   string = "CreatedResource"
	StartedCassandra                  string = "StartedCassandra"
	LabeledPodAsSeed                  string = "LabeledPodAsSeed"
	UnlabeledPodAsSeed                string = "UnlabeledPodAsSeed"
	LabeledRackResource               string = "LabeledRackResource"
	ScalingUpRack                     string = "ScalingUpRack"
	CreatedSuperuser                  string = "CreatedSuperuser" // deprecated
	CreatedUsers                      string = "CreatedUsers"
	FinishedReplaceNode               string = "FinishedReplaceNode"
	ReplacingNode                     string = "ReplacingNode"
	StartingCassandraAndReplacingNode string = "StartingCassandraAndReplacingNode"
	StartingCassandra                 string = "StartingCassandra"
)

type LoggingEventRecorder struct {
	record.EventRecorder
	ReqLogger logr.InfoLogger
}

func (r *LoggingEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	r.ReqLogger.Info(message, "reason", reason, "eventType", eventtype)
	r.EventRecorder.Event(object, eventtype, reason, message)
}

func (r *LoggingEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	r.ReqLogger.Info(fmt.Sprintf(messageFmt, args...), "reason", reason, "eventType", eventtype)
	r.EventRecorder.Eventf(object, eventtype, reason, messageFmt, args...)
}

func (r *LoggingEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	r.ReqLogger.Info(fmt.Sprintf(messageFmt, args...), "reason", reason, "eventType", eventtype)
	r.EventRecorder.AnnotatedEventf(object, annotations, eventtype, reason, messageFmt, args...)
}
