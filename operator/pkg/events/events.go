package events

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
	CreatedSuperuser                  string = "CreatedSuperuser"
	FinishedReplaceNode               string = "FinishedReplaceNode"
	ReplacingNode                     string = "ReplacingNode"
	StartingCassandraAndReplacingNode string = "StartingCassandraAndReplacingNode"
	StartingCassandra                 string = "StartingCassandra"
)
