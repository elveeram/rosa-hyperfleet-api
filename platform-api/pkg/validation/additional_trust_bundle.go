package validation

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

const (
	additionalTrustBundleField    = "spec.additionalTrustBundle"
	maxAdditionalTrustBundleBytes = 1 << 20
)

func validateAdditionalTrustBundle(fields map[string]any) *ValidationError {
	value, exists := fields[additionalTrustBundleField]
	if !exists || value == nil {
		return nil
	}

	bundle, ok := value.(string)
	if !ok {
		return &ValidationError{
			Field:  additionalTrustBundleField,
			Reason: "must be a PEM-encoded CA bundle",
		}
	}
	// An empty string is the supported way to clear the HostedCluster reference.
	if bundle == "" {
		return nil
	}
	if len(bundle) > maxAdditionalTrustBundleBytes {
		return &ValidationError{
			Field:  additionalTrustBundleField,
			Reason: fmt.Sprintf("must not exceed %d UTF-8 bytes", maxAdditionalTrustBundleBytes),
		}
	}
	if err := validatePEMTrustBundle(bundle); err != nil {
		return &ValidationError{
			Field:  additionalTrustBundleField,
			Reason: err.Error(),
		}
	}
	return nil
}

func validatePEMTrustBundle(bundle string) error {
	remaining := []byte(bundle)
	certCount := 0
	for {
		remaining = bytes.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		if !bytes.HasPrefix(remaining, []byte("-----BEGIN ")) {
			return fmt.Errorf("must contain only PEM-encoded CA certificates")
		}

		block, rest := pem.Decode(remaining)
		if block == nil || block.Type != "CERTIFICATE" {
			return fmt.Errorf("must contain only PEM-encoded CA certificates")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !cert.IsCA {
			return fmt.Errorf("must contain only valid PEM-encoded CA certificates")
		}
		certCount++
		remaining = rest
	}

	if certCount == 0 {
		return fmt.Errorf("must contain at least one PEM-encoded CA certificate")
	}
	return nil
}
