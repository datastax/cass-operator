// Copyright DataStax, Inc.
// Please see the included license file for details.

package utils

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func test_RangeInt(t *testing.T) {
	assert.Equal(t, []int{0, 2, 4, 6, 8}, RangeInt(0, 10, 2))
	assert.Equal(t, []int{0, 1, 2, 3, 4}, RangeInt(0, 5, 1))
	assert.Equal(t, []int{5, 8}, RangeInt(5, 10, 3))
}

type foo struct {
	a int
	b int
}

func Test_DeepEqualArrayIgnoreOrder(t *testing.T) {
	var aNil []int = nil
	var bNil []int = nil

	assert.True(t, DeepEqualArrayIgnoreOrder(
		[]foo{{1,2}, {3,4}, {5,6}},
		[]foo{{1,2}, {3,4}, {5,6}}))

	assert.True(t, DeepEqualArrayIgnoreOrder(
		[]foo{{1,2}, {3,4}, {5,6}},
		[]foo{{5,6}, {1,2}, {3,4}}))

	assert.True(t, DeepEqualArrayIgnoreOrder(
		aNil,
		bNil))

	assert.False(t, DeepEqualArrayIgnoreOrder(
		[]foo{{1,2}, {3,4}, {5,6}},
		[]foo{{5,6}, {1,2}}))

	assert.False(t, DeepEqualArrayIgnoreOrder(
		[]foo{{1,2}, {1,2}},
		[]foo{{5,6}, {1,2}}))

	assert.False(t, DeepEqualArrayIgnoreOrder(
		[]foo{{5,6}, {1,2}},
		[]foo{{1,2}, {1,2}}))

	assert.False(t, DeepEqualArrayIgnoreOrder(
		aNil,
		[]foo{{1,2}, {1,2}}))

	assert.False(t, DeepEqualArrayIgnoreOrder(
		[]foo{{5,6}, {1,2}},
		bNil))
}

func Test_mergeMap(t *testing.T) {
	type args struct {
		destination map[string]string
		sources     []map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "Same Map",
			args: args{
				destination: map[string]string{
					"foo": "bar",
				},
				sources: []map[string]string{
					{
						"foo": "bar",
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
			},
		}, {
			name: "Source missing key",
			args: args{
				destination: map[string]string{
					"foo": "bar",
				},
				sources: []map[string]string{
					{
						"foo": "bar",
						"key": "value",
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
				"key": "value",
			},
		}, {
			name: "Destination missing key",
			args: args{
				destination: map[string]string{
					"foo": "bar",
					"key": "value",
				},
				sources: []map[string]string{
					{
						"foo": "bar",
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
				"key": "value",
			},
		}, {
			name: "Empty Source",
			args: args{
				destination: map[string]string{
					"foo": "bar",
					"key": "value",
				},
				sources: []map[string]string{},
			},
			want: map[string]string{
				"foo": "bar",
				"key": "value",
			},
		}, {
			name: "Empty Destination",
			args: args{
				destination: map[string]string{},
				sources: []map[string]string{
					{
						"foo": "bar",
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
			},
		}, {
			name: "Differing values for key",
			args: args{
				destination: map[string]string{
					"foo": "bar",
					"key": "value",
				},
				sources: []map[string]string{
					{
						"foo": "bar",
						"key": "value2",
					},
				},
			},
			want: map[string]string{
				"foo": "bar",
				"key": "value2",
			},
		}, {
			name: "Multiple source maps",
			args: args{
				destination: map[string]string{
					"foo": "bar",
					"baz": "foobar",
				},
				sources: []map[string]string{
					{
						"foo": "qux",
						"waldo": "fred",

					},
					{
						"foo": "quux",
						"quuz": "flob",
					},
				},
			},
			want: map[string]string{
				"foo": "quux",
				"baz": "foobar",
				"waldo": "fred",
				"quuz": "flob",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MergeMap(tt.args.destination, tt.args.sources...)

			eq := reflect.DeepEqual(tt.args.destination, tt.want)
			if !eq {
				t.Errorf("mergeMap() = %v, want %v", tt.args.destination, tt.want)
			}
		})
	}
}

func TestSearchMap(t *testing.T) {
	type args struct {
		mapToSearch map[string]interface{}
		key         string
	}
	tests := []struct {
		name string
		args args
		want map[string]interface{}
	}{
		{
			name: "Happy Path",
			args: args{
				mapToSearch: map[string]interface{}{
					"key": map[string]interface{}{
						"foo": "bar",
					},
				},
				key: "key",
			},
			want: map[string]interface{}{
				"foo": "bar",
			},
		}, {
			name: "Deeply nested",
			args: args{
				mapToSearch: map[string]interface{}{
					"foo": "bar",
					"a": map[string]interface{}{
						"alpha": map[string]interface{}{
							"foo": "bar",
						},
						"alpha1": map[string]interface{}{
							"foo1": "bar1",
						},
					},
					"b": map[string]interface{}{
						"bravo": "bar",
						"bravo1": map[string]interface{}{
							"bravo111": map[string]interface{}{
								"key": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
					},
					"c": map[string]interface{}{
						"charlie": map[string]interface{}{
							"foo": "bar",
						},
						"charlie1": map[string]interface{}{
							"foo1": "bar1",
						},
					},
				},
				key: "key",
			},
			want: map[string]interface{}{
				"foo": "bar",
			},
		}, {
			name: "Key Not Found",
			args: args{
				mapToSearch: map[string]interface{}{
					"foo": "bar",
					"a": map[string]interface{}{
						"alpha": map[string]interface{}{
							"foo": "bar",
						},
						"alpha1": map[string]interface{}{
							"foo1": "bar1",
						},
					},
					"b": map[string]interface{}{
						"bravo": "bar",
						"bravo1": map[string]interface{}{
							"bravo111": map[string]interface{}{
								"wrong-key": map[string]interface{}{
									"foo": "bar",
								},
							},
						},
					},
					"c": map[string]interface{}{
						"charlie": map[string]interface{}{
							"foo": "bar",
						},
						"charlie1": map[string]interface{}{
							"foo1": "bar1",
						},
					},
				},
				key: "key",
			},
			want: map[string]interface{}{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchMap(tt.args.mapToSearch, tt.args.key)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchMap() got = %v, want %v", got, tt.want)
			}
		})
	}
}
