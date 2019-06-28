package utils

import (
	"reflect"
	"testing"
)

func Test_mergeMap(t *testing.T) {
	type args struct {
		destination *map[string]string
		source      map[string]string
	}
	tests := []struct {
		name string
		args args
		want *map[string]string
	}{
		{
			name: "Same Map",
			args: args{
				destination: &map[string]string{
					"foo": "bar",
				},
				source: map[string]string{
					"foo": "bar",
				},
			},
			want: &map[string]string{
				"foo": "bar",
			},
		}, {
			name: "Source missing key",
			args: args{
				destination: &map[string]string{
					"foo": "bar",
				},
				source: map[string]string{
					"foo": "bar",
					"key": "value",
				},
			},
			want: &map[string]string{
				"foo": "bar",
				"key": "value",
			},
		}, {
			name: "Destination missing key",
			args: args{
				destination: &map[string]string{
					"foo": "bar",
					"key": "value",
				},
				source: map[string]string{
					"foo": "bar",
				},
			},
			want: &map[string]string{
				"foo": "bar",
				"key": "value",
			},
		}, {
			name: "Empty Source",
			args: args{
				destination: &map[string]string{
					"foo": "bar",
					"key": "value",
				},
				source: map[string]string{},
			},
			want: &map[string]string{
				"foo": "bar",
				"key": "value",
			},
		}, {
			name: "Empty Destination",
			args: args{
				destination: &map[string]string{},
				source: map[string]string{
					"foo": "bar",
				},
			},
			want: &map[string]string{
				"foo": "bar",
			},
		}, {
			name: "Differing values for key",
			args: args{
				destination: &map[string]string{
					"foo": "bar",
					"key": "value",
				},
				source: map[string]string{
					"foo": "bar",
					"key": "value2",
				},
			},
			want: &map[string]string{
				"foo": "bar",
				"key": "value2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeMap(tt.args.destination, tt.args.source)

			eq := reflect.DeepEqual(tt.args.destination, tt.want)
			if !eq {
				t.Errorf("mergeMap() = %v, want %v", tt.args.destination, tt.want)
			}
		})
	}
}
