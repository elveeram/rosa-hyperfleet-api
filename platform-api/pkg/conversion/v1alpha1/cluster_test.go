package v1alpha1

import (
	"encoding/json"
	"strings"
	"testing"

	internal "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1"
	rest "github.com/openshift-online/rosa-hyperfleet-api/api/v1alpha1/public"
	configv1 "github.com/openshift/api/config/v1"
	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

func TestProjectClusterProjectsProxy(t *testing.T) {
	tests := []struct {
		name   string
		config *hypershiftv1beta1.ClusterConfiguration
		want   *rest.ClusterProxy
	}{
		{name: "no configuration"},
		{name: "no proxy", config: &hypershiftv1beta1.ClusterConfiguration{}},
		{
			name: "configured proxy",
			config: &hypershiftv1beta1.ClusterConfiguration{
				Proxy: &configv1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8443",
					NoProxy:    "localhost,127.0.0.1",
					TrustedCA:  configv1.ConfigMapNameReference{Name: "private-ca"},
				},
			},
			want: &rest.ClusterProxy{
				HTTPProxy:  "http://proxy.example.com:8080",
				HTTPSProxy: "https://proxy.example.com:8443",
				NoProxy:    "localhost,127.0.0.1",
			},
		},
		{
			name:   "empty proxy",
			config: &hypershiftv1beta1.ClusterConfiguration{Proxy: &configv1.ProxySpec{}},
			want:   &rest.ClusterProxy{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			crd := &internal.Cluster{}
			crd.Spec.HostedCluster.Configuration = tt.config

			got := ProjectCluster(crd)
			if got == nil {
				t.Fatal("ProjectCluster returned nil for a non-nil cluster")
			}
			if got.Proxy == nil && tt.want != nil {
				t.Fatal("expected projected proxy")
			}
			if got.Proxy != nil && tt.want == nil {
				t.Fatalf("expected no projected proxy, got %+v", got.Proxy)
			}
			if got.Proxy != nil && *got.Proxy != *tt.want {
				t.Errorf("projected proxy = %+v, want %+v", got.Proxy, tt.want)
			}
			if tt.config != nil && tt.config.Proxy != nil && tt.config.Proxy.TrustedCA.Name != "" {
				data, err := json.Marshal(got)
				if err != nil {
					t.Fatalf("marshal projected cluster: %v", err)
				}
				if strings.Contains(string(data), tt.config.Proxy.TrustedCA.Name) {
					t.Error("projected cluster unexpectedly exposed the trusted CA")
				}
			}
		})
	}
}

func TestProjectClusterNilInput(t *testing.T) {
	if got := ProjectCluster(nil); got != nil {
		t.Errorf("ProjectCluster(nil) = %+v, want nil", got)
	}
}

func TestUnprojectClusterPreservesProxyConfiguration(t *testing.T) {
	got := UnprojectCluster(&rest.ClusterSpec{
		HostedCluster: rest.HostedClusterSpecPassthrough{
			Configuration: &rest.ClusterConfiguration{
				Proxy: &rest.ProxyConfiguration{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8443",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
		},
	}, nil)
	if got == nil || got.HostedCluster.Configuration == nil || got.HostedCluster.Configuration.Proxy == nil {
		t.Fatal("expected proxy configuration to be preserved in the CRD")
	}

	proxy := got.HostedCluster.Configuration.Proxy
	if proxy.HTTPProxy != "http://proxy.example.com:8080" ||
		proxy.HTTPSProxy != "https://proxy.example.com:8443" ||
		proxy.NoProxy != "localhost,127.0.0.1" {
		t.Errorf("proxy configuration was not preserved: %+v", proxy)
	}
}
