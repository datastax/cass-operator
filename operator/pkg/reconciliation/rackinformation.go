// Copyright DataStax, Inc.
// Please see the included license file for details.

package reconciliation

// RackInformation contains an identifying name and a node count for a logical rack
type RackInformation struct {
	RackName  string
	NodeCount int
	SeedCount int
}
