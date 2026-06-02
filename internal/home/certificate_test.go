package home

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCertificatePathsSeparateCertificateIdentities(t *testing.T) {
	first, errFirst := defaultCertificatePaths(homeJWTClaims{
		ClusterID:     "cluster-a",
		CertificateID: "cert-1",
		CAFingerprint: strings.Repeat("a", 64),
	})
	if errFirst != nil {
		t.Fatalf("defaultCertificatePaths() error = %v", errFirst)
	}
	second, errSecond := defaultCertificatePaths(homeJWTClaims{
		ClusterID:     "cluster-a",
		CertificateID: "cert-2",
		CAFingerprint: strings.Repeat("a", 64),
	})
	if errSecond != nil {
		t.Fatalf("defaultCertificatePaths() error = %v", errSecond)
	}

	if first.ClientCert == second.ClientCert {
		t.Fatalf("client certificate path reused across certificate IDs: %s", first.ClientCert)
	}
	if first.ClientKey == second.ClientKey {
		t.Fatalf("client key path reused across certificate IDs: %s", first.ClientKey)
	}
	if first.CACert != second.CACert {
		t.Fatalf("CA path should be shared for the same cluster and CA fingerprint: %s != %s", first.CACert, second.CACert)
	}
}

func TestDefaultCertificatePathsSeparateCAFingerprints(t *testing.T) {
	first, errFirst := defaultCertificatePaths(homeJWTClaims{
		ClusterID:     "cluster-a",
		CertificateID: "cert-1",
		CAFingerprint: strings.Repeat("a", 64),
	})
	if errFirst != nil {
		t.Fatalf("defaultCertificatePaths() error = %v", errFirst)
	}
	second, errSecond := defaultCertificatePaths(homeJWTClaims{
		ClusterID:     "cluster-a",
		CertificateID: "cert-1",
		CAFingerprint: strings.Repeat("b", 64),
	})
	if errSecond != nil {
		t.Fatalf("defaultCertificatePaths() error = %v", errSecond)
	}

	if first.ClientCert == second.ClientCert {
		t.Fatalf("client certificate path reused across CA fingerprints: %s", first.ClientCert)
	}
	if first.CACert == second.CACert {
		t.Fatalf("CA path reused across CA fingerprints: %s", first.CACert)
	}
}

func TestCertificatePathPartSanitizesPathSeparators(t *testing.T) {
	paths, errPaths := defaultCertificatePaths(homeJWTClaims{
		ClusterID:     "../cluster/a",
		CertificateID: "cert/1",
		CAFingerprint: strings.Repeat("c", 64),
	})
	if errPaths != nil {
		t.Fatalf("defaultCertificatePaths() error = %v", errPaths)
	}

	for _, path := range []string{paths.ClientCert, paths.ClientKey, paths.CACert} {
		base := filepath.Base(path)
		if strings.Contains(base, "/") || strings.Contains(base, "..") {
			t.Fatalf("path is not sanitized: %s", path)
		}
	}
}
