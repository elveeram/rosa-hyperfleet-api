package validation

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestValidateAdditionalTrustBundle(t *testing.T) {
	ca := testPEMCertificate(t, true)
	leaf := testPEMCertificate(t, false)
	otherPEMType := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("key")}))
	invalidCertificate := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not a certificate")}))
	oversizedUTF8 := strings.Repeat("é", maxAdditionalTrustBundleBytes/2+1)

	tests := []struct {
		name    string
		fields  map[string]any
		wantErr bool
	}{
		{name: "field omitted"},
		{name: "null value is treated as omitted", fields: map[string]any{additionalTrustBundleField: nil}},
		{name: "empty string clears HostedCluster reference", fields: map[string]any{additionalTrustBundleField: ""}},
		{name: "valid CA certificate", fields: map[string]any{additionalTrustBundleField: ca}},
		{name: "multiple CA certificates", fields: map[string]any{additionalTrustBundleField: ca + "\n" + ca}},
		{name: "oversized UTF-8 value", fields: map[string]any{additionalTrustBundleField: oversizedUTF8}, wantErr: true},
		{name: "wrong JSON type", fields: map[string]any{additionalTrustBundleField: 123}, wantErr: true},
		{name: "non PEM text", fields: map[string]any{additionalTrustBundleField: "not a certificate"}, wantErr: true},
		{name: "malformed PEM block", fields: map[string]any{additionalTrustBundleField: "-----BEGIN CERTIFICATE-----\n%%%\n-----END CERTIFICATE-----"}, wantErr: true},
		{name: "non certificate PEM block", fields: map[string]any{additionalTrustBundleField: otherPEMType}, wantErr: true},
		{name: "invalid certificate DER", fields: map[string]any{additionalTrustBundleField: invalidCertificate}, wantErr: true},
		{name: "non CA certificate", fields: map[string]any{additionalTrustBundleField: leaf}, wantErr: true},
		{name: "whitespace only", fields: map[string]any{additionalTrustBundleField: " \n\t"}, wantErr: true},
		{name: "trailing non PEM data", fields: map[string]any{additionalTrustBundleField: ca + "unexpected"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdditionalTrustBundle(tt.fields)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateAdditionalTrustBundle() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && err.Field != additionalTrustBundleField {
				t.Errorf("error field = %q, want %q", err.Field, additionalTrustBundleField)
			}
		})
	}
}

func testPEMCertificate(t *testing.T, isCA bool) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}
