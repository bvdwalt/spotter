// Package photo fetches aircraft photos from planespotters.net by ICAO hex, with caching.
package photo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	positiveTTL = 24 * time.Hour
	negativeTTL = 6 * time.Hour
)

type Photo struct {
	Thumbnail    string `json:"thumbnail"`
	Large        string `json:"large"`
	Link         string `json:"link"`
	Photographer string `json:"photographer"`
}

type cached struct {
	photo   *Photo
	expires time.Time
}

type Client struct {
	BaseURL   string // e.g. https://api.planespotters.net/pub/photos/hex
	UserAgent string // planespotters requires a contact URL in the user agent
	Client    *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

func New(baseURL, version string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		UserAgent: "spotter/" + version + " (+https://github.com/bvdwalt/spotter)",
		Client:    &http.Client{Timeout: 10 * time.Second},
		cache:     make(map[string]cached),
	}
}

// Lookup returns the first photo for a hex code, or nil when there is none. Misses are cached too.
func (c *Client) Lookup(ctx context.Context, hex string) (*Photo, error) {
	hex = strings.ToLower(hex)
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.cache[hex]; ok && now.Before(e.expires) {
		c.mu.Unlock()
		return e.photo, nil
	}
	c.mu.Unlock()

	p, err := c.fetch(ctx, hex)
	if err != nil {
		return nil, err
	}
	ttl := negativeTTL
	if p != nil {
		ttl = positiveTTL
	}
	c.mu.Lock()
	c.cache[hex] = cached{photo: p, expires: now.Add(ttl)}
	c.mu.Unlock()
	return p, nil
}

type response struct {
	Photos []struct {
		Thumbnail      struct{ Src string } `json:"thumbnail"`
		ThumbnailLarge struct{ Src string } `json:"thumbnail_large"`
		Link           string               `json:"link"`
		Photographer   string               `json:"photographer"`
	} `json:"photos"`
	Error string `json:"error"`
}

func (c *Client) fetch(ctx context.Context, hex string) (*Photo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+hex, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return Parse(json.NewDecoder(resp.Body))
}

// Parse returns the first photo in a planespotters response, or nil when it has none.
func Parse(dec *json.Decoder) (*Photo, error) {
	var r response
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if r.Error != "" {
		return nil, fmt.Errorf("planespotters: %s", r.Error)
	}
	if len(r.Photos) == 0 {
		return nil, nil
	}
	p := r.Photos[0]
	return &Photo{Thumbnail: p.Thumbnail.Src, Large: p.ThumbnailLarge.Src, Link: p.Link, Photographer: p.Photographer}, nil
}
