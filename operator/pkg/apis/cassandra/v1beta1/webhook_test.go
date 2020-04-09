// Copyright DataStax, Inc.
// Please see the included license file for details.

package v1beta1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_ValidateSingleDatacenter(t *testing.T) {
	tests := []struct {
		name      string
		dc        *CassandraDatacenter
		want      string
		errString string
	}{
		{
			name: "Dse Valid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "6.8.0",
				},
			},
			errString: "",
		},
		{
			name: "Dse Invalid",
			dc: &CassandraDatacenter{
				ObjectMeta: metav1.ObjectMeta{
					Name: "exampleDC",
				},
				Spec: CassandraDatacenterSpec{
					ServerType:    "dse",
					ServerVersion: "4.8.0",
				},
			},
			errString: "CassandraDatacenter attempted to use unsupported DSE version '4.8.0'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSingleDatacenter(*tt.dc)
			if err == nil {
				if tt.errString != "" {
					t.Errorf("ValidateSingleDatacenter() err = %v, want %v", err, tt.errString)
				}
			} else {
				if err.Error() != tt.errString {
					t.Errorf("ValidateSingleDatacenter() err = %v, want %v", err, tt.errString)
				}
			}
		})
	}
}
