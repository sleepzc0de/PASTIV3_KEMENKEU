package utils

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"pasti-v3-backend/config"
)

// NewSSOHTTPClient membuat http.Client yang eksplisit mendukung proxy sistem,
// dengan timeout terpisah untuk setiap fase koneksi supaya lebih mudah didiagnosis.
func NewSSOHTTPClient() *http.Client {
	timeout := time.Duration(config.Cfg.HTTPClientTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // otomatis baca HTTPS_PROXY/HTTP_PROXY dari environment
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		IdleConnTimeout:       90 * time.Second,
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// DoWithRetry menjalankan request HTTP dengan retry otomatis (exponential backoff)
// untuk menangani kegagalan sementara seperti timeout jaringan.
func DoWithRetry(client *http.Client, req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		start := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(start)

		if err == nil {
			if attempt > 1 {
				log.Printf("[SSO INFO] Request berhasil pada percobaan ke-%d (durasi: %s)", attempt, duration)
			}
			return resp, nil
		}

		lastErr = err
		log.Printf("[SSO WARN] Percobaan ke-%d gagal (durasi: %s): %v", attempt, duration, err)

		if attempt < maxRetries {
			backoff := time.Duration(attempt) * 2 * time.Second
			log.Printf("[SSO INFO] Menunggu %s sebelum retry...", backoff)
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}

			// Request body sudah terpakai di percobaan sebelumnya, perlu di-reset kalau ada body
			if req.GetBody != nil {
				newBody, bodyErr := req.GetBody()
				if bodyErr == nil {
					req.Body = newBody
				}
			}
		}
	}

	return nil, fmt.Errorf("gagal setelah %d percobaan: %w", maxRetries, lastErr)
}

// NewRequestWithTimeout helper untuk membuat request dengan context timeout eksplisit
func NewRequestWithTimeout(method, url string, timeoutSeconds int) (*http.Request, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	return req, cancel, err
}
