package captureproxy

import (
	"crypto/tls"
	"sync"
)

type memoryCertStore struct {
	mu    sync.Mutex
	certs map[string]*tls.Certificate
}

func newMemoryCertStore() *memoryCertStore {
	return &memoryCertStore{certs: make(map[string]*tls.Certificate)}
}

func (s *memoryCertStore) Fetch(hostname string, gen func() (*tls.Certificate, error)) (*tls.Certificate, error) {
	s.mu.Lock()
	if cert, ok := s.certs[hostname]; ok {
		s.mu.Unlock()
		return cert, nil
	}
	s.mu.Unlock()

	cert, err := gen()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.certs[hostname] = cert
	s.mu.Unlock()
	return cert, nil
}
