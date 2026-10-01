package pathbind_test

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	"github.com/openshift-online/rosa-hyperfleet-api/clientset/pathbind"
)

// clusterInput is a flat consumer struct representing cluster creation inputs.
type clusterInput struct {
	Name    string `hfsdk:"metadata.name"`
	Version string `hfsdk:"spec.hostedCluster.release.image"`
	Region  string `hfsdk:"spec.hostedCluster.platform.aws.region"`
	VPC     string `hfsdk:"spec.hostedCluster.platform.aws.cloudProviderConfig.vpc"`
	Zone    string `hfsdk:"spec.hostedCluster.platform.aws.cloudProviderConfig.zone"`
	Subnet  string `hfsdk:"spec.hostedCluster.platform.aws.cloudProviderConfig.subnet.id"`
	Derived string `hfsdk:"-"`
}

// clusterOutput is a flat consumer struct for reading cluster state back.
type clusterOutput struct {
	Name    string       `hfsdk:"metadata.name"`
	Version string       `hfsdk:"spec.hostedCluster.release.image"`
	Region  string       `hfsdk:"spec.hostedCluster.platform.aws.region"`
	Phase   string       `hfsdk:"status.phase"`
	Expiry  *metav1.Time `hfsdk:"spec.expirationTimestamp"`
}

// nodePoolInput is a flat consumer struct for nodepool creation.
type nodePoolInput struct {
	Name         string `hfsdk:"metadata.name"`
	ClusterName  string `hfsdk:"spec.nodePool.clusterName"`
	Replicas     *int32 `hfsdk:"spec.nodePool.replicas"`
	InstanceType string `hfsdk:"spec.nodePool.platform.aws.instanceType"`
	SubnetID     string `hfsdk:"spec.nodePool.platform.aws.subnet.id"`
}

type nodePoolAutoscalingInput struct {
	Min *int32 `hfsdk:"spec.nodePool.autoScaling.min"`
	Max *int32 `hfsdk:"spec.nodePool.autoScaling.max"`
}

type nodePoolAdditionalFieldsInput struct {
	Config           []corev1.LocalObjectReference `hfsdk:"spec.nodePool.config"`
	NodeDrainTimeout string                        `hfsdk:"spec.nodePool.nodeDrainTimeout"`
	Taints           []hypershiftv1beta1.Taint     `hfsdk:"spec.nodePool.taints"`
	TuningConfig     []corev1.LocalObjectReference `hfsdk:"spec.nodePool.tuningConfig"`
}

func TestExpand_ClusterDirectMappings(t *testing.T) {
	ctx := context.Background()
	input := clusterInput{
		Name:    "my-cluster",
		Version: "quay.io/openshift-release-dev/ocp-release:4.18.0",
		Region:  "us-east-1",
		VPC:     "vpc-abc123",
		Zone:    "us-east-1a",
		Subnet:  "subnet-def456",
		Derived: "should be ignored",
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}

	if cluster.Name != "my-cluster" {
		t.Errorf("Name: got %q, want %q", cluster.Name, "my-cluster")
	}
	if cluster.Spec.HostedCluster.Release.Image != "quay.io/openshift-release-dev/ocp-release:4.18.0" {
		t.Errorf("Release.Image: got %q", cluster.Spec.HostedCluster.Release.Image)
	}
	if cluster.Spec.HostedCluster.Platform.AWS == nil {
		t.Fatal("Platform.AWS is nil; should have been allocated")
	}
	if cluster.Spec.HostedCluster.Platform.AWS.Region != "us-east-1" {
		t.Errorf("Region: got %q", cluster.Spec.HostedCluster.Platform.AWS.Region)
	}
	cfg := cluster.Spec.HostedCluster.Platform.AWS.CloudProviderConfig
	if cfg == nil {
		t.Fatal("CloudProviderConfig is nil; should have been allocated")
	}
	if cfg.VPC != "vpc-abc123" {
		t.Errorf("VPC: got %q", cfg.VPC)
	}
	if cfg.Zone != "us-east-1a" {
		t.Errorf("Zone: got %q", cfg.Zone)
	}
	if cfg.Subnet == nil || cfg.Subnet.ID == nil {
		t.Fatal("Subnet or Subnet.ID is nil")
	}
	if *cfg.Subnet.ID != "subnet-def456" {
		t.Errorf("Subnet.ID: got %q", *cfg.Subnet.ID)
	}
	// Derived field must not affect the cluster struct.
	if cluster.Annotations != nil {
		t.Error("unexpected annotations set")
	}
}

func TestExpand_NilPointerIntermediatesAllocated(t *testing.T) {
	ctx := context.Background()
	// Only set Region — which requires AWS to be allocated.
	input := struct {
		Region string `hfsdk:"spec.hostedCluster.platform.aws.region"`
	}{Region: "eu-west-1"}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Spec.HostedCluster.Platform.AWS == nil {
		t.Error("AWS should have been allocated")
	}
	if cluster.Spec.HostedCluster.Platform.AWS.Region != "eu-west-1" {
		t.Errorf("Region: got %q", cluster.Spec.HostedCluster.Platform.AWS.Region)
	}
}

func TestExpand_EmptyStringSkipped(t *testing.T) {
	ctx := context.Background()
	input := clusterInput{
		Name:   "filled",
		Region: "", // empty — must not allocate AWS or touch Region
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Spec.HostedCluster.Platform.AWS != nil {
		t.Error("AWS should not have been allocated for an empty Region")
	}
}

func TestExpand_PointerInt32(t *testing.T) {
	ctx := context.Background()
	replicas := int32(3)
	input := nodePoolInput{
		Name:         "pool-1",
		ClusterName:  "my-cluster",
		Replicas:     &replicas,
		InstanceType: "m7i.xlarge",
		SubnetID:     "subnet-np1",
	}

	np := &v1alpha1.NodePool{}
	if err := pathbind.Expand(ctx, input, np); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if np.Name != "pool-1" {
		t.Errorf("Name: got %q", np.Name)
	}
	if np.Spec.NodePool.Replicas == nil {
		t.Fatal("Replicas should not be nil")
	}
	if *np.Spec.NodePool.Replicas != 3 {
		t.Errorf("Replicas: got %d", *np.Spec.NodePool.Replicas)
	}
	if np.Spec.NodePool.Platform.AWS == nil {
		t.Fatal("AWS should have been allocated")
	}
	if np.Spec.NodePool.Platform.AWS.InstanceType != "m7i.xlarge" {
		t.Errorf("InstanceType: got %q", np.Spec.NodePool.Platform.AWS.InstanceType)
	}
	if np.Spec.NodePool.Platform.AWS.Subnet.ID == nil || *np.Spec.NodePool.Platform.AWS.Subnet.ID != "subnet-np1" {
		t.Errorf("Subnet.ID: got %v", np.Spec.NodePool.Platform.AWS.Subnet.ID)
	}
}

func TestExpand_NilPointerFieldSkipped(t *testing.T) {
	ctx := context.Background()
	input := nodePoolInput{
		Name:     "pool-x",
		Replicas: nil, // nil pointer — Replicas must stay nil in the result
	}

	np := &v1alpha1.NodePool{}
	if err := pathbind.Expand(ctx, input, np); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if np.Spec.NodePool.Replicas != nil {
		t.Errorf("Replicas should be nil when input is nil, got %v", np.Spec.NodePool.Replicas)
	}
}

func TestExpand_NodePoolAutoScalingRequest(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		min  int32
		max  int32
	}{
		{name: "normal bounds", min: 1, max: 3},
		{name: "explicit scale from zero", min: 0, max: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max := tt.min, tt.max
			np := &v1alpha1.NodePool{}
			if err := pathbind.Expand(ctx, nodePoolAutoscalingInput{Min: &min, Max: &max}, np); err != nil {
				t.Fatalf("Expand: %v", err)
			}
			if np.Spec.NodePool.AutoScaling == nil {
				t.Fatal("AutoScaling is nil after expansion")
			}
			if np.Spec.NodePool.AutoScaling.Min == nil || *np.Spec.NodePool.AutoScaling.Min != tt.min {
				t.Fatalf("AutoScaling.Min = %v, want %d", np.Spec.NodePool.AutoScaling.Min, tt.min)
			}
			if np.Spec.NodePool.AutoScaling.Max != tt.max {
				t.Fatalf("AutoScaling.Max = %d, want %d", np.Spec.NodePool.AutoScaling.Max, tt.max)
			}
			if np.Spec.NodePool.Replicas != nil {
				t.Fatalf("Replicas = %d with autoscaling set, want nil", *np.Spec.NodePool.Replicas)
			}

			assertNodePoolRequestScaling(t, np, true, tt.min, tt.max, nil)
		})
	}
}

func TestExpand_NodePoolAutoScalingUpdateRequest(t *testing.T) {
	ctx := context.Background()
	oldMin := int32(1)
	np := &v1alpha1.NodePool{}
	np.Spec.NodePool.AutoScaling = &hypershiftv1beta1.NodePoolAutoScaling{Min: &oldMin, Max: 3}

	min, max := int32(2), int32(6)
	if err := pathbind.Expand(ctx, nodePoolAutoscalingInput{Min: &min, Max: &max}, np); err != nil {
		t.Fatalf("Expand update: %v", err)
	}
	if got := np.Spec.NodePool.AutoScaling; got == nil || got.Min == nil || *got.Min != min || got.Max != max {
		t.Fatalf("AutoScaling after update expansion = %#v, want min=%d max=%d", got, min, max)
	}
	assertNodePoolRequestScaling(t, np, true, min, max, nil)
}

func TestExpand_NodePoolFixedReplicasRequest(t *testing.T) {
	ctx := context.Background()
	replicas := int32(3)
	input := struct {
		Replicas *int32 `hfsdk:"spec.nodePool.replicas"`
	}{Replicas: &replicas}

	np := &v1alpha1.NodePool{}
	if err := pathbind.Expand(ctx, input, np); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if np.Spec.NodePool.AutoScaling != nil {
		t.Fatal("AutoScaling is set for a fixed replica request")
	}
	if np.Spec.NodePool.Replicas == nil || *np.Spec.NodePool.Replicas != replicas {
		t.Fatalf("Replicas = %v, want %d", np.Spec.NodePool.Replicas, replicas)
	}
	assertNodePoolRequestScaling(t, np, false, 0, 0, &replicas)
}

func TestNodePoolAdditionalFieldsExpandAndFlatten(t *testing.T) {
	ctx := context.Background()
	input := nodePoolAdditionalFieldsInput{
		Config:           []corev1.LocalObjectReference{{Name: "kubelet-config"}},
		NodeDrainTimeout: "5m",
		Taints: []hypershiftv1beta1.Taint{{
			Key:    "dedicated",
			Value:  "batch",
			Effect: corev1.TaintEffectNoSchedule,
		}},
		TuningConfig: []corev1.LocalObjectReference{{Name: "performance-tuning"}},
	}

	np := &v1alpha1.NodePool{}
	if err := pathbind.Expand(ctx, input, np); err != nil {
		t.Fatalf("Expand: %v", err)
	}

	got := np.Spec.NodePool
	if !reflect.DeepEqual(got.Config, input.Config) {
		t.Errorf("Config = %#v, want %#v", got.Config, input.Config)
	}
	if got.NodeDrainTimeout == nil || got.NodeDrainTimeout.Duration != 5*time.Minute {
		t.Errorf("NodeDrainTimeout = %v, want 5m", got.NodeDrainTimeout)
	}
	if !reflect.DeepEqual(got.Taints, input.Taints) {
		t.Errorf("Taints = %#v, want %#v", got.Taints, input.Taints)
	}
	if !reflect.DeepEqual(got.TuningConfig, input.TuningConfig) {
		t.Errorf("TuningConfig = %#v, want %#v", got.TuningConfig, input.TuningConfig)
	}

	flattened := &nodePoolAdditionalFieldsInput{}
	if err := pathbind.Flatten(ctx, np, flattened); err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if !reflect.DeepEqual(flattened.Config, input.Config) {
		t.Errorf("Flattened Config = %#v, want %#v", flattened.Config, input.Config)
	}
	if flattened.NodeDrainTimeout != (5 * time.Minute).String() {
		t.Errorf("Flattened NodeDrainTimeout = %q, want %q", flattened.NodeDrainTimeout, (5 * time.Minute).String())
	}
	if !reflect.DeepEqual(flattened.Taints, input.Taints) {
		t.Errorf("Flattened Taints = %#v, want %#v", flattened.Taints, input.Taints)
	}
	if !reflect.DeepEqual(flattened.TuningConfig, input.TuningConfig) {
		t.Errorf("Flattened TuningConfig = %#v, want %#v", flattened.TuningConfig, input.TuningConfig)
	}
}

func assertNodePoolRequestScaling(t *testing.T, np *v1alpha1.NodePool, wantAutoScaling bool, wantMin, wantMax int32, wantReplicas *int32) {
	t.Helper()
	body, err := json.Marshal(np)
	if err != nil {
		t.Fatalf("marshal NodePool request: %v", err)
	}
	var request struct {
		Spec struct {
			NodePool map[string]json.RawMessage `json:"nodePool"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatalf("decode NodePool request: %v", err)
	}
	autoScalingJSON, hasAutoScaling := request.Spec.NodePool["autoScaling"]
	if hasAutoScaling != wantAutoScaling {
		t.Fatalf("request autoScaling present = %t, want %t; body: %s", hasAutoScaling, wantAutoScaling, body)
	}
	replicasJSON, hasReplicas := request.Spec.NodePool["replicas"]
	if wantReplicas == nil {
		if hasReplicas {
			t.Fatalf("request includes replicas with autoscaling enabled; body: %s", body)
		}
	} else {
		var gotReplicas int32
		if !hasReplicas {
			t.Fatalf("request omits fixed replicas; body: %s", body)
		}
		if err := json.Unmarshal(replicasJSON, &gotReplicas); err != nil {
			t.Fatalf("decode replicas request: %v", err)
		}
		if gotReplicas != *wantReplicas {
			t.Fatalf("request replicas = %d, want %d; body: %s", gotReplicas, *wantReplicas, body)
		}
	}
	if !wantAutoScaling {
		return
	}
	var autoScaling struct {
		Min *int32 `json:"min"`
		Max int32  `json:"max"`
	}
	if err := json.Unmarshal(autoScalingJSON, &autoScaling); err != nil {
		t.Fatalf("decode autoScaling request: %v", err)
	}
	if autoScaling.Min == nil || *autoScaling.Min != wantMin {
		t.Fatalf("request autoScaling.min = %v, want %d; body: %s", autoScaling.Min, wantMin, body)
	}
	if autoScaling.Max != wantMax {
		t.Fatalf("request autoScaling.max = %d, want %d; body: %s", autoScaling.Max, wantMax, body)
	}
}

func TestExpand_InvalidPath(t *testing.T) {
	ctx := context.Background()
	bad := struct {
		X string `hfsdk:"spec.hostedCluster.nonexistentField"`
	}{X: "value"}

	err := pathbind.Expand(ctx, bad, &v1alpha1.Cluster{})
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestExpand_SkipDashTag(t *testing.T) {
	ctx := context.Background()
	input := struct {
		Ignored string `hfsdk:"-"`
		Name    string `hfsdk:"metadata.name"`
	}{Ignored: "should be ignored", Name: "real-name"}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Name != "real-name" {
		t.Errorf("Name: got %q", cluster.Name)
	}
}

func TestFlatten_ClusterDirectMappings(t *testing.T) {
	ctx := context.Background()
	subnetID := "subnet-abc"
	cluster := &v1alpha1.Cluster{}
	cluster.Name = "my-cluster"
	cluster.Spec.HostedCluster.Release.Image = "quay.io/openshift-release-dev/ocp-release:4.18.0"
	cluster.Spec.HostedCluster.Platform.AWS = &hypershiftv1beta1.AWSPlatformSpec{
		Region: "us-east-1",
		CloudProviderConfig: &hypershiftv1beta1.AWSCloudProviderConfig{
			VPC:    "vpc-abc123",
			Zone:   "us-east-1a",
			Subnet: &hypershiftv1beta1.AWSResourceReference{ID: &subnetID},
		},
	}
	cluster.Status.Phase = v1alpha1.ClusterPhaseReady

	out := &clusterOutput{}
	if err := pathbind.Flatten(ctx, cluster, out); err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if out.Name != "my-cluster" {
		t.Errorf("Name: got %q", out.Name)
	}
	if out.Version != "quay.io/openshift-release-dev/ocp-release:4.18.0" {
		t.Errorf("Version: got %q", out.Version)
	}
	if out.Region != "us-east-1" {
		t.Errorf("Region: got %q", out.Region)
	}
	if out.Phase != string(v1alpha1.ClusterPhaseReady) {
		t.Errorf("Phase: got %q", out.Phase)
	}
}

func TestFlatten_NilIntermediateSkipped(t *testing.T) {
	ctx := context.Background()
	// Platform.AWS is nil — Region should remain empty, no panic.
	cluster := &v1alpha1.Cluster{}
	cluster.Name = "no-aws"

	out := &clusterOutput{}
	if err := pathbind.Flatten(ctx, cluster, out); err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if out.Name != "no-aws" {
		t.Errorf("Name: got %q", out.Name)
	}
	if out.Region != "" {
		t.Errorf("Region should be empty when AWS is nil, got %q", out.Region)
	}
}

func TestRoundTrip_MetavTime(t *testing.T) {
	ctx := context.Background()
	ts := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

	type withExpiry struct {
		Expiry string `hfsdk:"spec.expirationTimestamp"`
	}

	in := withExpiry{Expiry: ts.Format(time.RFC3339)}
	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, in, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Spec.ExpirationTimestamp == nil {
		t.Fatal("ExpirationTimestamp should not be nil")
	}
	if !cluster.Spec.ExpirationTimestamp.UTC().Equal(ts) {
		t.Errorf("time mismatch: got %v, want %v", cluster.Spec.ExpirationTimestamp.UTC(), ts)
	}

	// Flatten back.
	out := &withExpiry{}
	if err := pathbind.Flatten(ctx, cluster, out); err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if out.Expiry != ts.Format(time.RFC3339) {
		t.Errorf("Expiry round-trip: got %q, want %q", out.Expiry, ts.Format(time.RFC3339))
	}
}

func TestExpand_PointerSourceValue(t *testing.T) {
	ctx := context.Background()
	// Passing a pointer to the source struct should work identically.
	input := &clusterInput{
		Name:   "ptr-cluster",
		Region: "ap-southeast-1",
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand via pointer: %v", err)
	}
	if cluster.Name != "ptr-cluster" {
		t.Errorf("Name: got %q", cluster.Name)
	}
}

// TestExpand_SliceFromJSONString verifies that a consumer string field holding
// a JSON array is correctly unmarshaled into the SDK slice field.
// e.g. additionalAllowedPrincipals: []string
func TestExpand_SliceFromJSONString(t *testing.T) {
	ctx := context.Background()
	input := struct {
		Principals string `hfsdk:"spec.hostedCluster.platform.aws.additionalAllowedPrincipals"`
	}{
		Principals: `["arn:aws:iam::123:role/role-a","arn:aws:iam::123:role/role-b"]`,
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Spec.HostedCluster.Platform.AWS == nil {
		t.Fatal("AWS should have been allocated")
	}
	got := cluster.Spec.HostedCluster.Platform.AWS.AdditionalAllowedPrincipals
	if len(got) != 2 {
		t.Fatalf("AdditionalAllowedPrincipals: got %d entries, want 2", len(got))
	}
	if got[0] != "arn:aws:iam::123:role/role-a" {
		t.Errorf("entry[0]: got %q", got[0])
	}
}

// TestExpand_SliceOfStructFromJSONString verifies that a consumer string field
// holding a JSON array of objects is unmarshaled into the SDK slice of structs.
// e.g. resourceTags: []AWSResourceTag{Key, Value}
func TestExpand_SliceOfStructFromJSONString(t *testing.T) {
	ctx := context.Background()
	input := struct {
		Tags string `hfsdk:"spec.hostedCluster.platform.aws.resourceTags"`
	}{
		Tags: `[{"key":"env","value":"prod"},{"key":"team","value":"platform"}]`,
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Spec.HostedCluster.Platform.AWS == nil {
		t.Fatal("AWS should have been allocated")
	}
	tags := cluster.Spec.HostedCluster.Platform.AWS.ResourceTags
	if len(tags) != 2 {
		t.Fatalf("ResourceTags: got %d entries, want 2", len(tags))
	}
	if tags[0].Key != "env" {
		t.Errorf("tags[0].Key: got %q, want %q", tags[0].Key, "env")
	}
}

// TestExpand_SliceDirectAssignment verifies that a consumer slice field of the
// matching SDK type is copied directly without JSON encoding.
func TestExpand_SliceDirectAssignment(t *testing.T) {
	ctx := context.Background()
	input := struct {
		Principals []string `hfsdk:"spec.hostedCluster.platform.aws.additionalAllowedPrincipals"`
	}{
		Principals: []string{"arn:aws:iam::123:role/role-x"},
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	got := cluster.Spec.HostedCluster.Platform.AWS.AdditionalAllowedPrincipals
	if len(got) != 1 || got[0] != "arn:aws:iam::123:role/role-x" {
		t.Errorf("got %v", got)
	}
}

// TestFlatten_SliceToJSONString verifies that an SDK slice field is JSON-marshaled
// into a consumer string field.
func TestFlatten_SliceToJSONString(t *testing.T) {
	ctx := context.Background()
	cluster := &v1alpha1.Cluster{}
	cluster.Spec.HostedCluster.Platform.AWS = &hypershiftv1beta1.AWSPlatformSpec{
		AdditionalAllowedPrincipals: []string{
			"arn:aws:iam::123:role/role-a",
			"arn:aws:iam::123:role/role-b",
		},
	}

	out := &struct {
		Principals string `hfsdk:"spec.hostedCluster.platform.aws.additionalAllowedPrincipals"`
	}{}
	if err := pathbind.Flatten(ctx, cluster, out); err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	want := `["arn:aws:iam::123:role/role-a","arn:aws:iam::123:role/role-b"]`
	if out.Principals != want {
		t.Errorf("Principals: got %q, want %q", out.Principals, want)
	}
}

// TestRoundTrip_SliceField verifies that a JSON string round-trips through
// Expand → Flatten without data loss.
func TestRoundTrip_SliceField(t *testing.T) {
	ctx := context.Background()
	type input struct {
		Tags string `hfsdk:"spec.hostedCluster.platform.aws.resourceTags"`
	}
	original := `[{"key":"env","value":"prod"}]`

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input{Tags: original}, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}

	out := &input{}
	if err := pathbind.Flatten(ctx, cluster, out); err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if out.Tags != original {
		t.Errorf("round-trip: got %q, want %q", out.Tags, original)
	}
}

// TestExpand_ZeroInt32Skipped verifies that nil pointers are skipped (absent),
// while non-nil pointers with explicit zero values are preserved and forwarded.
// Downstream validators (e.g. hypershift operator) reject semantically invalid zeros.
func TestExpand_ZeroInt32Skipped(t *testing.T) {
	ctx := context.Background()
	zero := int32(0)

	// Test: nil Port pointer is skipped
	clusterInput := struct {
		Port *int32 `hfsdk:"spec.hostedCluster.networking.apiServer.port"`
	}{
		Port: nil, // nil pointer is skipped
	}
	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, clusterInput, cluster); err != nil {
		t.Fatalf("Expand cluster: %v", err)
	}
	if as := cluster.Spec.HostedCluster.Networking.APIServer; as != nil {
		t.Error("APIServer should not be allocated when Port is nil")
	}

	// Test: non-nil Replicas pointer with zero value is forwarded
	npInput := struct {
		Replicas *int32 `hfsdk:"spec.nodePool.replicas"`
	}{
		Replicas: &zero, // non-nil pointer to zero is forwarded (replicas=0 is valid)
	}
	np := &v1alpha1.NodePool{}
	if err := pathbind.Expand(ctx, npInput, np); err != nil {
		t.Fatalf("Expand NodePool: %v", err)
	}
	// Replicas should be 0 because the input pointer is non-nil with explicit zero value.
	if np.Spec.NodePool.Replicas == nil {
		t.Error("replicas should be allocated for explicit zero pointer")
	} else if *np.Spec.NodePool.Replicas != 0 {
		t.Errorf("replicas should be 0, got %v", *np.Spec.NodePool.Replicas)
	}
}

// TestExpand_NonZeroInt32Forwarded verifies that a non-zero *int32 IS forwarded.
func TestExpand_NonZeroInt32Forwarded(t *testing.T) {
	ctx := context.Background()
	port := int32(6443)
	input := struct {
		Port *int32 `hfsdk:"spec.hostedCluster.networking.apiServer.port"`
	}{Port: &port}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if cluster.Spec.HostedCluster.Networking.APIServer == nil {
		t.Fatal("APIServer must be allocated for non-zero port")
	}
	if cluster.Spec.HostedCluster.Networking.APIServer.Port == nil || *cluster.Spec.HostedCluster.Networking.APIServer.Port != 6443 {
		t.Errorf("port: got %v, want 6443", cluster.Spec.HostedCluster.Networking.APIServer.Port)
	}
}

func TestExpand_EmptySliceSkipped(t *testing.T) {
	ctx := context.Background()
	input := struct {
		Principals []string `hfsdk:"spec.hostedCluster.platform.aws.additionalAllowedPrincipals"`
	}{
		Principals: []string{},
	}

	cluster := &v1alpha1.Cluster{}
	if err := pathbind.Expand(ctx, input, cluster); err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// Empty slice → AWS should not be allocated.
	if cluster.Spec.HostedCluster.Platform.AWS != nil {
		t.Error("AWS should not have been allocated for an empty slice")
	}
}

func TestExpand_NonPointerDstRejected(t *testing.T) {
	ctx := context.Background()
	input := clusterInput{Name: "test"}

	// Passing a non-pointer struct should be rejected
	cluster := v1alpha1.Cluster{} // non-pointer
	err := pathbind.Expand(ctx, input, cluster)
	if err == nil {
		t.Error("expected error when dst is not a pointer, got nil")
	}
	if !strings.Contains(err.Error(), "must be a non-nil pointer to a struct") {
		t.Errorf("got unexpected error: %v", err)
	}
}

func TestFlatten_NonPointerDstRejected(t *testing.T) {
	ctx := context.Background()
	cluster := &v1alpha1.Cluster{}
	cluster.Name = "test"

	// Passing a non-pointer struct should be rejected
	output := clusterOutput{} // non-pointer
	err := pathbind.Flatten(ctx, cluster, output)
	if err == nil {
		t.Error("expected error when dst is not a pointer, got nil")
	}
	if !strings.Contains(err.Error(), "must be a non-nil pointer to a struct") {
		t.Errorf("got unexpected error: %v", err)
	}
}

func TestExpand_Int64ToInt32Overflow(t *testing.T) {
	ctx := context.Background()
	// int64 value larger than int32 max (2147483647)
	largeInt := int64(2147483648)
	input := struct {
		Value int64 `hfsdk:"metadata.name"` // This will be converted to string
	}{Value: largeInt}

	cluster := &v1alpha1.Cluster{}
	// This should error because the conversion would overflow
	err := pathbind.Expand(ctx, input, cluster)
	if err == nil {
		t.Error("expected error for int64→string conversion on numeric value, got nil")
	}
	if !strings.Contains(err.Error(), "cannot convert numeric") {
		t.Errorf("got unexpected error: %v", err)
	}
}

func TestExpand_NumericToStringRejected(t *testing.T) {
	ctx := context.Background()
	// Try to convert a numeric value to string (would reinterpret as code point)
	input := struct {
		Port int32 `hfsdk:"metadata.name"` // numeric being converted to string field
	}{Port: 65}

	cluster := &v1alpha1.Cluster{}
	err := pathbind.Expand(ctx, input, cluster)
	if err == nil {
		t.Error("expected error for numeric→string conversion, got nil")
	}
	if !strings.Contains(err.Error(), "cannot convert numeric") {
		t.Errorf("got unexpected error: %v", err)
	}
}
