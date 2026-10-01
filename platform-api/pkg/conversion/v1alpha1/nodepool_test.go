package v1alpha1

import (
	"reflect"
	"testing"
	"time"

	crd "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectNodePoolObservedReplicas(t *testing.T) {
	replicas := int32(0)
	projected := ProjectNodePool(&crd.NodePool{
		Status: crd.NodePoolStatus{Replicas: &replicas},
	})
	if projected.Status.Replicas == nil || *projected.Status.Replicas != 0 {
		t.Fatalf("projected replicas = %v, want pointer to zero", projected.Status.Replicas)
	}

	unknown := ProjectNodePool(&crd.NodePool{})
	if unknown.Status.Replicas != nil {
		t.Fatalf("projected replicas = %v, want nil", unknown.Status.Replicas)
	}
}

func TestProjectNodePoolExposesPassthroughFields(t *testing.T) {
	min := int32(1)
	drainTimeout := &metav1.Duration{Duration: 5 * time.Minute}
	source := &crd.NodePool{
		Spec: crd.NodePoolSpec{
			NodePool: crd.NodePoolSpecPassthrough{
				Management: hypershiftv1beta1.NodePoolManagement{
					AutoRepair:  true,
					UpgradeType: hypershiftv1beta1.UpgradeTypeReplace,
				},
				AutoScaling:      &hypershiftv1beta1.NodePoolAutoScaling{Min: &min, Max: 3},
				Config:           []corev1.LocalObjectReference{{Name: "kubelet-config"}},
				NodeDrainTimeout: drainTimeout,
				NodeLabels:       map[string]string{"node-role.kubernetes.io/worker": ""},
				Taints: []hypershiftv1beta1.Taint{{
					Key:    "dedicated",
					Value:  "batch",
					Effect: corev1.TaintEffectNoSchedule,
				}},
				TuningConfig: []corev1.LocalObjectReference{{Name: "performance-tuning"}},
			},
		},
	}

	projected := ProjectNodePool(source)
	got := projected.Spec.NodePool
	want := source.Spec.NodePool
	if !reflect.DeepEqual(got.Management, want.Management) {
		t.Errorf("Management = %#v, want %#v", got.Management, want.Management)
	}
	if !reflect.DeepEqual(got.AutoScaling, want.AutoScaling) {
		t.Errorf("AutoScaling = %#v, want %#v", got.AutoScaling, want.AutoScaling)
	}
	if !reflect.DeepEqual(got.Config, want.Config) {
		t.Errorf("Config = %#v, want %#v", got.Config, want.Config)
	}
	if !reflect.DeepEqual(got.NodeDrainTimeout, want.NodeDrainTimeout) {
		t.Errorf("NodeDrainTimeout = %#v, want %#v", got.NodeDrainTimeout, want.NodeDrainTimeout)
	}
	if !reflect.DeepEqual(got.NodeLabels, want.NodeLabels) {
		t.Errorf("NodeLabels = %#v, want %#v", got.NodeLabels, want.NodeLabels)
	}
	if !reflect.DeepEqual(got.Taints, want.Taints) {
		t.Errorf("Taints = %#v, want %#v", got.Taints, want.Taints)
	}
	if !reflect.DeepEqual(got.TuningConfig, want.TuningConfig) {
		t.Errorf("TuningConfig = %#v, want %#v", got.TuningConfig, want.TuningConfig)
	}
}
