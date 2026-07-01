package captureproxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

type caFiles struct {
	CertPath string
	KeyPath  string
	CertPEM  []byte
	Cert     tls.Certificate
}

func ensureCA(dir string) (caFiles, error) {
	certPath := filepath.Join(dir, "mygardenworld-capture-ca.crt")
	keyPath := filepath.Join(dir, "mygardenworld-capture-ca.key")
	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			return loadCA(certPath, keyPath)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return caFiles{}, fmt.Errorf("mkdir ca dir: %w", err)
	}
	certPEM, keyPEM, err := generateCA()
	if err != nil {
		return caFiles{}, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return caFiles{}, fmt.Errorf("write ca cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return caFiles{}, fmt.Errorf("write ca key: %w", err)
	}
	return loadCA(certPath, keyPath)
}

func loadCA(certPath, keyPath string) (caFiles, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return caFiles{}, fmt.Errorf("read ca cert: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return caFiles{}, fmt.Errorf("read ca key: %w", err)
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return caFiles{}, fmt.Errorf("parse ca keypair: %w", err)
	}
	if cert.Leaf == nil {
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return caFiles{}, fmt.Errorf("parse ca leaf: %w", err)
		}
		cert.Leaf = leaf
	}
	return caFiles{CertPath: certPath, KeyPath: keyPath, CertPEM: certPEM, Cert: cert}, nil
}

func generateCA() ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ca key: %w", err)
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("generate ca serial: %w", err)
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "mygardenworld capture proxy",
			Organization: []string{"mygardenworld local capture"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(3650 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create ca cert: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM, nil
}
