package dsenode

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	datastaxv1alpha1 "github.com/riptano/dse-operator/operator/pkg/apis/datastax/v1alpha1"
)

func Test_shouldReconcilePod(t *testing.T) {
	type args struct {
		metaNew metav1.Object
		metaOld metav1.Object
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Seed, IP Changed",
			args: args{
				metaNew: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.SeedNodeLabel: "true",
							datastaxv1alpha1.RackLabel:     "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.1",
					},
				},
				metaOld: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.SeedNodeLabel: "true",
							datastaxv1alpha1.RackLabel:     "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.0",
					},
				},
			},
			want: true,
		},
		{
			name: "Seed, IP Same",
			args: args{
				metaNew: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.SeedNodeLabel: "true",
							datastaxv1alpha1.RackLabel:     "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.1",
					},
				},
				metaOld: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.SeedNodeLabel: "true",
							datastaxv1alpha1.RackLabel:     "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.1",
					},
				},
			},
			want: false,
		},
		{
			name: "Not Seed, IP Same",
			args: args{
				metaNew: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.RackLabel: "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.1",
					},
				},
				metaOld: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.RackLabel: "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.0",
					},
				},
			},
			want: false,
		},
		{
			name: "Not Seed, IP Changed",
			args: args{
				metaNew: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.RackLabel: "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.1",
					},
				},
				metaOld: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							datastaxv1alpha1.RackLabel: "default",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "127.0.0.0",
					},
				},
			},
			want: false,
		},
		{
			name: "Not owned by operator, IP Changed",
			args: args{
				metaNew: &corev1.Pod{
					Status: corev1.PodStatus{
						PodIP: "127.0.0.1",
					},
				},
				metaOld: &corev1.Pod{
					Status: corev1.PodStatus{
						PodIP: "127.0.0.0",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldReconcilePod(tt.args.metaNew, tt.args.metaOld); got != tt.want {
				t.Errorf("shouldReconcilePod() = %v, want %v", got, tt.want)
			}
		})
	}
}
