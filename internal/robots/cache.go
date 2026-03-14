package robots

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/temoto/robotstxt"
)

type Cache struct {
	hc  *http.Client
	lru *expirable.LRU[string, *robotstxt.RobotsData]
	mu  sync.RWMutex
	ua  string
}

func NewCache(hc *http.Client, ua string, size int, ttl time.Duration) *Cache {
	return &Cache{
		hc:  hc,
		lru: expirable.NewLRU[string, *robotstxt.RobotsData](size, nil, ttl),
		ua:  ua,
	}
}

// SetUA updates the user-agent string in a thread-safe manner.
func (c *Cache) SetUA(ua string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ua = ua
}

func (c *Cache) Get(ctx context.Context, host string) (*robotstxt.RobotsData, error) {
	if v, ok := c.lru.Get(host); ok {
		return v, nil
	}

	c.mu.RLock()
	ua := c.ua
	c.mu.RUnlock()

	urls := []string{"https://" + host + "/robots.txt", "http://" + host + "/robots.txt"}
	for _, ru := range urls {
		req, _ := http.NewRequestWithContext(ctx, "GET", ru, nil)
		req.Header.Set("User-Agent", ua)
		resp, err := c.hc.Do(req)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 404 {
			rd, _ := robotstxt.FromBytes([]byte{})
			c.lru.Add(host, rd)
			return rd, nil
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			rd, _ := robotstxt.FromBytes(b)
			c.lru.Add(host, rd)
			return rd, nil
		}
	}
	rd, _ := robotstxt.FromBytes([]byte{})
	c.lru.Add(host, rd)
	return rd, nil
}

func Allowed(rd *robotstxt.RobotsData, ua, path string) bool {
	if rd == nil {
		return true
	}
	g := rd.FindGroup(ua)
	if g == nil {
		g = rd.FindGroup("*")
	}
	if g == nil {
		return true
	}
	return g.Test(path)
}

func ShouldSkipByTLD(host string, excluded []string) bool {
	for _, t := range excluded {
		if strings.HasSuffix(host, "."+t) || host == t {
			return true
		}
	}
	return false
}
