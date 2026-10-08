package conversion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateResponseProjection(t *testing.T) {
	for _, tt := range []struct {
		name       string
		projection string
		wantError  string
	}{
		{name: "proxy", projection: "proxy"},
		{name: "unknown projection", projection: "unknown", wantError: `unsupported REST response projection "unknown" on Cluster`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			inputDir := filepath.Join(tmpDir, "input")
			if err := os.MkdirAll(inputDir, 0755); err != nil {
				t.Fatalf("create input directory: %v", err)
			}
			fixture := strings.ReplaceAll(responseProjectionFixture, "PROJECTION", tt.projection)
			if err := os.WriteFile(filepath.Join(inputDir, "cluster.go"), []byte(fixture), 0644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			outputDir := filepath.Join(tmpDir, "platform", "conversion", "v1alpha1")
			restDir := filepath.Join(tmpDir, "api", "public")
			gen := NewGenerator("v1alpha1", "example.com/project/api", []string{inputDir}, outputDir)
			gen.OutputPackage = "example.com/project/platform/conversion"
			gen.RESTOutputDir = restDir
			gen.RESTImportPath = "example.com/project/api/public"

			err := gen.Generate()
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("Generate() error = %v, want it to contain %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Generate(): %v", err)
			}

			restTypes := readGeneratedFile(t, filepath.Join(restDir, "cluster_types.go"))
			if !strings.Contains(restTypes, "Proxy *ClusterProxy") || !strings.Contains(restTypes, `json:"proxy,omitempty"`) {
				t.Errorf("generated Cluster REST type is missing the proxy projection field:\n%s", restTypes)
			}

			conversions := readGeneratedFile(t, filepath.Join(outputDir, "cluster.go"))
			for _, want := range []string{
				"if config := crd.Spec.HostedCluster.Configuration; config != nil && config.Proxy != nil",
				"out.Proxy = &rest.ClusterProxy{",
				"HTTPProxy:  config.Proxy.HTTPProxy,",
				"HTTPSProxy: config.Proxy.HTTPSProxy,",
				"NoProxy:    config.Proxy.NoProxy,",
			} {
				if !strings.Contains(conversions, want) {
					t.Errorf("generated conversion is missing %q:\n%s", want, conversions)
				}
			}
		})
	}
}

const responseProjectionFixture = `package fixture

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +hyperfleet:rest-response-projection=PROJECTION

// Cluster is a fixture resource for response projection generation.
type Cluster struct {
	metav1.TypeMeta ` + "`json:\",inline\"`" + `
	metav1.ObjectMeta ` + "`json:\"metadata,omitempty\"`" + `
	Spec ClusterSpec ` + "`json:\"spec\"`" + `
	Status ClusterStatus ` + "`json:\"status,omitempty\"`" + `
}

type ClusterSpec struct {
	HostedCluster HostedClusterSpecPassthrough ` + "`json:\"hostedCluster\"`" + `
}

type HostedClusterSpecPassthrough struct {
	Configuration *ClusterConfiguration ` + "`json:\"configuration,omitempty\"`" + `
}

type ClusterConfiguration struct {
	Proxy *ProxyConfiguration ` + "`json:\"proxy,omitempty\"`" + `
}

type ProxyConfiguration struct {
	HTTPProxy string ` + "`json:\"httpProxy,omitempty\"`" + `
	HTTPSProxy string ` + "`json:\"httpsProxy,omitempty\"`" + `
	NoProxy string ` + "`json:\"noProxy,omitempty\"`" + `
}

type ClusterStatus struct {
	Phase string ` + "`json:\"phase,omitempty\"`" + `
}
`

func readGeneratedFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated file %s: %v", path, err)
	}
	return string(data)
}

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("v1alpha1", "test/pkg", []string{"/test/dir"}, "/test/output")

	if gen.APIVersion != "v1alpha1" {
		t.Errorf("Expected APIVersion v1alpha1, got %s", gen.APIVersion)
	}
	if gen.CRDPackage != "test/pkg" {
		t.Errorf("Expected CRDPackage test/pkg, got %s", gen.CRDPackage)
	}
	if gen.OutputDir != "/test/output" {
		t.Errorf("Expected OutputDir /test/output, got %s", gen.OutputDir)
	}
	if len(gen.InputDirs) != 1 || gen.InputDirs[0] != "/test/dir" {
		t.Errorf("Expected InputDirs [/test/dir], got %v", gen.InputDirs)
	}
	if gen.knownTypes == nil {
		t.Error("Expected knownTypes map to be initialized")
	}
	if gen.typeInfos == nil {
		t.Error("Expected typeInfos map to be initialized")
	}
}

func TestBuildFieldPath(t *testing.T) {
	gen := NewGenerator("v1alpha1", "test", []string{}, "")

	tests := []struct {
		name     string
		typeName string
		jsonName string
		want     string
	}{
		{
			name:     "Spec type",
			typeName: "ClusterSpec",
			jsonName: "displayName",
			want:     "spec.displayName",
		},
		{
			name:     "Status type",
			typeName: "ClusterStatus",
			jsonName: "state",
			want:     "status.state",
		},
		{
			name:     "HostedCluster passthrough",
			typeName: "HostedClusterSpecPassthrough",
			jsonName: "platform",
			want:     "spec.hostedCluster.platform",
		},
		{
			name:     "NodePool passthrough",
			typeName: "NodePoolSpecPassthrough",
			jsonName: "release",
			want:     "spec.nodePool.release",
		},
		{
			name:     "Other type",
			typeName: "ClusterReference",
			jsonName: "name",
			want:     "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gen.buildFieldPath(tt.typeName, tt.jsonName)
			if got != tt.want {
				t.Errorf("buildFieldPath(%q, %q) = %q, want %q", tt.typeName, tt.jsonName, got, tt.want)
			}
		})
	}
}

func TestExtractJSONTag(t *testing.T) {
	// This would require creating AST field structures
	// For now, just test the basic logic is correct
	t.Skip("Requires AST field creation - tested via integration")
}

func TestExprToString(t *testing.T) {
	// This would require creating AST expression structures
	// For now, just test the basic logic is correct
	t.Skip("Requires AST expression creation - tested via integration")
}

// Note: needsFieldPrefix and makeUniqueFieldName are private helper methods
// They are tested indirectly via the integration tests that verify
// the generated ServiceSetFields struct has correctly prefixed field names

func TestEnsureDir(t *testing.T) {
	gen := NewGenerator("v1alpha1", "test", []string{}, "")

	// Create temp dir for test
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "test", "nested", "dir")

	// Directory should not exist yet
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Fatalf("Test directory should not exist yet")
	}

	// Create it
	if err := gen.ensureDir(testDir); err != nil {
		t.Fatalf("ensureDir failed: %v", err)
	}

	// Should exist now
	if stat, err := os.Stat(testDir); err != nil {
		t.Fatalf("Directory was not created: %v", err)
	} else if !stat.IsDir() {
		t.Fatal("Path exists but is not a directory")
	}

	// Calling again should be idempotent
	if err := gen.ensureDir(testDir); err != nil {
		t.Fatalf("ensureDir should be idempotent: %v", err)
	}
}
