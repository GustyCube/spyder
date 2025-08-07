package httpclient

import (
	"crypto/tls"
	"net/http"
	time "time"
)

func Default() *http.Client {
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
		DisableCompression:    false,
		MaxIdleConns:          1024,
		MaxConnsPerHost:       128,
		MaxIdleConnsPerHost:   64,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   15 * time.Second,
	}
}
