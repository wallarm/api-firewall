package proxy

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateTestCACert creates a self-signed CA certificate PEM file for testing.
func generateTestCACert(t *testing.T, dir string) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPath := filepath.Join(dir, "ca.pem")
	f, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	defer f.Close()

	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("failed to write PEM: %v", err)
	}

	return certPath
}

func TestBuildTLSConfig(t *testing.T) {
	tests := []struct {
		name       string
		insecure   bool
		rootCA     string
		setupFunc  func(t *testing.T) string // returns rootCA path
		wantErr    bool
		errContain string
		checkFunc  func(t *testing.T, cfg *tls.Config)
	}{
		{
			name:     "defaults - no custom CA, secure",
			insecure: false,
			rootCA:   "",
			checkFunc: func(t *testing.T, cfg *tls.Config) {
				if cfg.InsecureSkipVerify {
					t.Error("expected InsecureSkipVerify to be false")
				}
				if cfg.RootCAs == nil {
					t.Error("expected non-nil RootCAs")
				}
			},
		},
		{
			name:     "insecure mode",
			insecure: true,
			rootCA:   "",
			checkFunc: func(t *testing.T, cfg *tls.Config) {
				if !cfg.InsecureSkipVerify {
					t.Error("expected InsecureSkipVerify to be true")
				}
			},
		},
		{
			name:     "valid custom CA",
			insecure: false,
			setupFunc: func(t *testing.T) string {
				return generateTestCACert(t, t.TempDir())
			},
			checkFunc: func(t *testing.T, cfg *tls.Config) {
				if cfg.RootCAs == nil {
					t.Error("expected non-nil RootCAs")
				}
			},
		},
		{
			name:       "nonexistent CA file",
			insecure:   false,
			rootCA:     "/nonexistent/path/ca.pem",
			wantErr:    true,
			errContain: "failed to read root CA",
		},
		{
			name:     "invalid PEM content",
			insecure: false,
			setupFunc: func(t *testing.T) string {
				dir := t.TempDir()
				path := filepath.Join(dir, "bad.pem")
				if err := os.WriteFile(path, []byte("not a valid PEM"), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
				return path
			},
			wantErr:    true,
			errContain: "failed to append root CA",
		},
		{
			name:     "insecure with custom CA",
			insecure: true,
			setupFunc: func(t *testing.T) string {
				return generateTestCACert(t, t.TempDir())
			},
			checkFunc: func(t *testing.T, cfg *tls.Config) {
				if !cfg.InsecureSkipVerify {
					t.Error("expected InsecureSkipVerify to be true")
				}
				if cfg.RootCAs == nil {
					t.Error("expected non-nil RootCAs even in insecure mode")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCA := tt.rootCA
			if tt.setupFunc != nil {
				rootCA = tt.setupFunc(t)
			}

			cfg, err := BuildTLSConfig(tt.insecure, rootCA)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContain != "" && !contains(err.Error(), tt.errContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg == nil {
				t.Fatal("expected non-nil config")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
