package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeImage(t *testing.T) {
	type args struct {
		repo    string
		version string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test empty inputs",
			args: args{
				repo:    "",
				version: "",
			},
			want: "datastax/dse-server:6.7.3",
		},
		{
			name: "test different public version",
			args: args{
				repo:    "",
				version: "6.8",
			},
			want: "datastax/dse-server:6.8",
		},
		{
			name: "test private repo server",
			args: args{
				repo:    "datastax.jfrog.io/secret-debug-image/dse-server",
				version: "",
			},
			want: "datastax.jfrog.io/secret-debug-image/dse-server:6.7.3",
		},
		{
			name: "test fully custom params",
			args: args{
				repo:    "jfrog.io:6789/dse-server-team/dse-server",
				version: "master.20190605.123abc",
			},
			want: "jfrog.io:6789/dse-server-team/dse-server:master.20190605.123abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeImage(tt.args.repo, tt.args.version); got != tt.want {
				t.Errorf("makeImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDseDatacenter_GetServerImage(t *testing.T) {
	type fields struct {
		TypeMeta   metav1.TypeMeta
		ObjectMeta metav1.ObjectMeta
		Spec       DseDatacenterSpec
		Status     DseDatacenterStatus
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "simple test",
			fields: fields{
				Spec: DseDatacenterSpec{
					Repository: "jfrog.io:6789/dse-server-team/dse-server",
					Version: "master.20190605.123abc",
				},
			},
			want: "jfrog.io:6789/dse-server-team/dse-server:master.20190605.123abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dc := &DseDatacenter{
				TypeMeta:   tt.fields.TypeMeta,
				ObjectMeta: tt.fields.ObjectMeta,
				Spec:       tt.fields.Spec,
				Status:     tt.fields.Status,
			}
			if got := dc.GetServerImage(); got != tt.want {
				t.Errorf("DseDatacenter.GetServerImage() = %v, want %v", got, tt.want)
			}
		})
	}
}
