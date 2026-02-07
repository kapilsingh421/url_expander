package expander
import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)
// Common errors
var (
	ErrEmptyURL         = errors.New("empty URL provided")
	ErrInvalidURL       = errors.New("invalid URL format")
	ErrTooManyRedirects = errors.New("too many redirects")
	ErrTimeout          = errors.New("request timeout")
	ErrNoRedirect       = errors.New("URL does not redirect")
)
// RedirectInfo contains information about a single redirect
type RedirectInfo struct {
	FromURL    string
	ToURL      string
	StatusCode int
	Duration   time.Duration
}
// ExpansionResult contains the result of URL expansion
type ExpansionResult struct {
	OriginalURL  string
	ExpandedURL  string
	FinalURL     string
	RedirectChain []RedirectInfo
	TotalTime    time.Duration
	Hops         int
	Error        error
}
// Config holds configuration for the URL expander
type Config struct {
	// Timeout for the entire expansion operation
	Timeout time.Duration
	
	// MaxRedirects is the maximum number of redirects to follow
	MaxRedirects int
	
	// UserAgent to use for requests
	UserAgent string
	
	// FollowMetaRefresh attempts to follow HTML meta refresh redirects
	FollowMetaRefresh bool
	
	// SkipTLSVerify skips TLS certificate verification (use with caution)
	SkipTLSVerify bool
	
	// CustomHeaders to add to requests
	CustomHeaders map[string]string
	
	// RetryCount for failed requests
	RetryCount int
	
	// RetryDelay between retries
	RetryDelay time.Duration
}
// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Timeout:           30 * time.Second,
		MaxRedirects:      20,
		UserAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		FollowMetaRefresh: false,
		SkipTLSVerify:     false,
		CustomHeaders:     make(map[string]string),
		RetryCount:        2,
		RetryDelay:        500 * time.Millisecond,
	}
}
// Expander is the main URL expander
type Expander struct {
	config *Config
	client *http.Client
	mu     sync.RWMutex
}
// New creates a new Expander with the given configuration
func New(config *Config) *Expander {
	if config == nil {
		config = DefaultConfig()
	}
	
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.SkipTLSVerify,
		},
	}
	
	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Prevent automatic redirect following
			return http.ErrUseLastResponse
		},
	}
	
	return &Expander{
		config: config,
		client: client,
	}
}
// Expand expands a short URL and returns detailed expansion result
func (e *Expander) Expand(ctx context.Context, shortURL string) *ExpansionResult {
	startTime := time.Now()
	result := &ExpansionResult{
		OriginalURL:   shortURL,
		RedirectChain: make([]RedirectInfo, 0),
	}
	
	// Validate and normalize URL
	normalizedURL, err := e.validateAndNormalizeURL(shortURL)
	if err != nil {
		result.Error = err
		result.TotalTime = time.Since(startTime)
		return result
	}
	
	// Follow redirects
	finalURL, err := e.followRedirects(ctx, normalizedURL, result)
	result.FinalURL = finalURL
	result.ExpandedURL = finalURL
	result.Error = err
	result.TotalTime = time.Since(startTime)
	return result
}
// validateAndNormalizeURL validates and normalizes the input URL
func (e *Expander) validateAndNormalizeURL(shortURL string) (string, error) {
	if shortURL == "" {
		return "", ErrEmptyURL
	}
	
	normalizedURL, err := normalizeURL(shortURL)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	return normalizedURL, nil
}
// followRedirects follows redirects and populates the result
func (e *Expander) followRedirects(ctx context.Context, startURL string, result *ExpansionResult) (string, error) {
	currentURL := startURL
	visited := make(map[string]bool)
	
	for i := 0; i < e.config.MaxRedirects; i++ {
		if err := ctx.Err(); err != nil {
			return currentURL, err
		}
		
		if visited[currentURL] {
			return currentURL, fmt.Errorf("redirect loop detected at: %s", currentURL)
		}
		visited[currentURL] = true
		
		nextURL, done, err := e.processRedirect(ctx, currentURL, result)
		if err != nil {
			return currentURL, err
		}
		if done {
			return nextURL, nil
		}
		currentURL = nextURL
	}
	
	return currentURL, ErrTooManyRedirects
}
// processRedirect processes a single redirect hop
func (e *Expander) processRedirect(ctx context.Context, currentURL string, result *ExpansionResult) (string, bool, error) {
	resp, hopDuration, err := e.makeRequestWithRetry(ctx, currentURL)
	if err != nil {
		if len(result.RedirectChain) > 0 {
			return currentURL, true, nil
		}
		return "", false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if !isRedirect(resp.StatusCode) {
		return currentURL, true, nil
	}
	
	nextURL, err := e.extractRedirectURL(currentURL, resp)
	if err != nil {
		return currentURL, true, err
	}
	
	result.RedirectChain = append(result.RedirectChain, RedirectInfo{
		FromURL:    currentURL,
		ToURL:      nextURL,
		StatusCode: resp.StatusCode,
		Duration:   hopDuration,
	})
	result.Hops++
	
	return nextURL, false, nil
}
// makeRequestWithRetry makes a request with retries
func (e *Expander) makeRequestWithRetry(ctx context.Context, targetURL string) (*http.Response, time.Duration, error) {
	hopStart := time.Now()
	var resp *http.Response
	var err error
	
	for retry := 0; retry <= e.config.RetryCount; retry++ {
		resp, err = e.makeRequest(ctx, targetURL)
		if err == nil {
			return resp, time.Since(hopStart), nil
		}
		if retry < e.config.RetryCount {
			time.Sleep(e.config.RetryDelay)
		}
	}
	return nil, time.Since(hopStart), err
}
// extractRedirectURL extracts and validates the redirect URL from response
func (e *Expander) extractRedirectURL(currentURL string, resp *http.Response) (string, error) {
	location := resp.Header.Get("Location")
	if location == "" {
		return "", errors.New("redirect without Location header")
	}
	return resolveURL(currentURL, location)
}
// ExpandSimple returns just the expanded URL string
func (e *Expander) ExpandSimple(ctx context.Context, shortURL string) (string, error) {
	result := e.Expand(ctx, shortURL)
	if result.Error != nil && result.FinalURL == "" {
		return "", result.Error
	}
	return result.FinalURL, nil
}
// ExpandBatch expands multiple URLs concurrently
func (e *Expander) ExpandBatch(ctx context.Context, urls []string, concurrency int) []*ExpansionResult {
	if concurrency <= 0 {
		concurrency = 5
	}
	
	results := make([]*ExpansionResult, len(urls))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)
	
	for i, u := range urls {
		wg.Add(1)
		go func(index int, url string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			results[index] = e.Expand(ctx, url)
		}(i, u)
	}
	
	wg.Wait()
	return results
}
// makeRequest creates and executes an HTTP request
func (e *Expander) makeRequest(ctx context.Context, targetURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers
	req.Header.Set("User-Agent", e.config.UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	
	for key, value := range e.config.CustomHeaders {
		req.Header.Set(key, value)
	}
	
	resp, err := e.client.Do(req)
	if err != nil {
		// Try GET if HEAD fails
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", e.config.UserAgent)
		req.Header.Set("Accept", "*/*")
		for key, value := range e.config.CustomHeaders {
			req.Header.Set(key, value)
		}
		resp, err = e.client.Do(req)
		if err != nil {
			return nil, err
		}
	}
	
	return resp, nil
}
// isRedirect checks if the status code is a redirect
func isRedirect(statusCode int) bool {
	return statusCode == http.StatusMovedPermanently ||
		statusCode == http.StatusFound ||
		statusCode == http.StatusSeeOther ||
		statusCode == http.StatusTemporaryRedirect ||
		statusCode == http.StatusPermanentRedirect
}
// normalizeURL ensures URL has a scheme and is valid
func normalizeURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	
	if rawURL == "" {
		return "", errors.New("empty URL")
	}
	
	// Add scheme if missing
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}
	
	// Validate URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	
	if parsed.Host == "" {
		return "", errors.New("missing host")
	}
	
	return parsed.String(), nil
}
// resolveURL resolves a possibly relative URL against a base URL
func resolveURL(baseURL, refURL string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	
	ref, err := url.Parse(refURL)
	if err != nil {
		return "", err
	}
	
	resolved := base.ResolveReference(ref)
	return resolved.String(), nil
}
// IsShortURL attempts to detect if a URL is likely a short URL
func IsShortURL(rawURL string) bool {
	shortURLDomains := []string{
		"bit.ly", "bitly.com",
		"tinyurl.com",
		"t.co",
		"goo.gl",
		"ow.ly",
		"is.gd", "v.gd",
		"buff.ly",
		"adf.ly",
		"j.mp",
		"shorturl.at",
		"tiny.cc",
		"rb.gy",
		"cutt.ly",
		"t.ly",
		"soo.gd",
		"s.id",
		"clck.ru",
		"rebrand.ly",
		"bl.ink",
		"short.io",
	}
	
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	
	host := strings.ToLower(parsed.Host)
	for _, domain := range shortURLDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	
	// Heuristic: short path length often indicates short URL
	if len(parsed.Path) > 0 && len(parsed.Path) <= 10 {
		return true
	}
	
	return false
}
// Close closes the expander and releases resources
func (e *Expander) Close() {
	e.client.CloseIdleConnections()
}
