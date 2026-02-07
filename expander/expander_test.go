package expander

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExpandURL_SingleRedirect(t *testing.T) {
	// Create test server that redirects
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Final destination"))
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	exp := New(nil)
	defer exp.Close()

	result := exp.Expand(context.Background(), redirectServer.URL)

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}

	if result.FinalURL != finalServer.URL {
		t.Errorf("Expected final URL %s, got %s", finalServer.URL, result.FinalURL)
	}

	if result.Hops != 1 {
		t.Errorf("Expected 1 hop, got %d", result.Hops)
	}
}

func TestExpandURL_MultipleRedirects(t *testing.T) {
	// Create chain: server1 -> server2 -> server3 (final)
	server3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server3.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server3.URL, http.StatusFound)
	}))
	defer server2.Close()

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server2.URL, http.StatusMovedPermanently)
	}))
	defer server1.Close()

	exp := New(nil)
	defer exp.Close()

	result := exp.Expand(context.Background(), server1.URL)

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}

	if result.FinalURL != server3.URL {
		t.Errorf("Expected final URL %s, got %s", server3.URL, result.FinalURL)
	}

	if result.Hops != 2 {
		t.Errorf("Expected 2 hops, got %d", result.Hops)
	}

	if len(result.RedirectChain) != 2 {
		t.Errorf("Expected 2 redirects in chain, got %d", len(result.RedirectChain))
	}
}

func TestExpandURL_NoRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := New(nil)
	defer exp.Close()

	result := exp.Expand(context.Background(), server.URL)

	if result.Error != nil {
		t.Errorf("Expected no error, got: %v", result.Error)
	}

	if result.FinalURL != server.URL {
		t.Errorf("Expected same URL %s, got %s", server.URL, result.FinalURL)
	}

	if result.Hops != 0 {
		t.Errorf("Expected 0 hops, got %d", result.Hops)
	}
}

func TestExpandURL_EmptyURL(t *testing.T) {
	exp := New(nil)
	defer exp.Close()

	result := exp.Expand(context.Background(), "")

	if result.Error != ErrEmptyURL {
		t.Errorf("Expected ErrEmptyURL, got: %v", result.Error)
	}
}

func TestExpandURL_InvalidURL(t *testing.T) {
	exp := New(nil)
	defer exp.Close()

	result := exp.Expand(context.Background(), "not-a-valid-url-at-all")

	// Should try to normalize and add https://
	// If it can't connect, that's expected
	if result.FinalURL == "" && result.Error == nil {
		t.Error("Expected either a URL or an error")
	}
}

func TestExpandURL_RedirectLoop(t *testing.T) {
	var server1, server2 *httptest.Server

	server1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server2.URL, http.StatusFound)
	}))
	defer server1.Close()

	server2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, server1.URL, http.StatusFound)
	}))
	defer server2.Close()

	exp := New(nil)
	defer exp.Close()

	result := exp.Expand(context.Background(), server1.URL)

	if result.Error == nil {
		t.Error("Expected error for redirect loop")
	}
}

func TestExpandURL_TooManyRedirects(t *testing.T) {
	config := DefaultConfig()
	config.MaxRedirects = 3

	// Create a server that always redirects to itself with different path
	counter := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter++
		http.Redirect(w, r, r.URL.String()+"x", http.StatusFound)
	}))
	defer server.Close()

	exp := New(config)
	defer exp.Close()

	result := exp.Expand(context.Background(), server.URL)

	if result.Error != ErrTooManyRedirects {
		t.Errorf("Expected ErrTooManyRedirects, got: %v", result.Error)
	}
}

func TestExpandURL_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := New(nil)
	defer exp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := exp.Expand(ctx, server.URL)

	if result.Error == nil {
		t.Error("Expected context cancellation error")
	}
}

func TestExpandSimple(t *testing.T) {
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	exp := New(nil)
	defer exp.Close()

	url, err := exp.ExpandSimple(context.Background(), redirectServer.URL)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if url != finalServer.URL {
		t.Errorf("Expected %s, got %s", finalServer.URL, url)
	}
}

func TestExpandBatch(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	exp := New(nil)
	defer exp.Close()

	urls := []string{server1.URL, server2.URL}
	results := exp.ExpandBatch(context.Background(), urls, 2)

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for i, result := range results {
		if result.Error != nil {
			t.Errorf("Result %d: unexpected error: %v", i, result.Error)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		hasError bool
	}{
		{"https://example.com", "https://example.com", false},
		{"http://example.com", "http://example.com", false},
		{"example.com", "https://example.com", false},
		{"example.com/path", "https://example.com/path", false},
		{"", "", true},
	}

	for _, test := range tests {
		result, err := normalizeURL(test.input)
		
		if test.hasError && err == nil {
			t.Errorf("normalizeURL(%q): expected error", test.input)
		}
		
		if !test.hasError && err != nil {
			t.Errorf("normalizeURL(%q): unexpected error: %v", test.input, err)
		}
		
		if !test.hasError && result != test.expected {
			t.Errorf("normalizeURL(%q): expected %q, got %q", test.input, test.expected, result)
		}
	}
}

func TestIsShortURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://bit.ly/abc123", true},
		{"https://tinyurl.com/xyz", true},
		{"https://t.co/abc", true},
		{"https://goo.gl/maps/xyz", true},
		{"https://www.google.com", false},
		{"https://example.com/very/long/path/here", false},
		{"https://shorturl.at/abc", true},
	}

	for _, test := range tests {
		result := IsShortURL(test.url)
		if result != test.expected {
			t.Errorf("IsShortURL(%q): expected %v, got %v", test.url, test.expected, result)
		}
	}
}

func TestIsRedirect(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{200, false},
		{301, true},
		{302, true},
		{303, true},
		{304, false},
		{307, true},
		{308, true},
		{400, false},
		{404, false},
		{500, false},
	}

	for _, test := range tests {
		result := isRedirect(test.code)
		if result != test.expected {
			t.Errorf("isRedirect(%d): expected %v, got %v", test.code, test.expected, result)
		}
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		base     string
		ref      string
		expected string
	}{
		{"https://example.com/path", "/new", "https://example.com/new"},
		{"https://example.com/path", "https://other.com", "https://other.com"},
		{"https://example.com/path/", "relative", "https://example.com/path/relative"},
	}

	for _, test := range tests {
		result, err := resolveURL(test.base, test.ref)
		if err != nil {
			t.Errorf("resolveURL(%q, %q): unexpected error: %v", test.base, test.ref, err)
		}
		if result != test.expected {
			t.Errorf("resolveURL(%q, %q): expected %q, got %q", test.base, test.ref, test.expected, result)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if config.MaxRedirects != 20 {
		t.Errorf("Expected max redirects 20, got %d", config.MaxRedirects)
	}

	if config.RetryCount != 2 {
		t.Errorf("Expected retry count 2, got %d", config.RetryCount)
	}
}

// Benchmark tests
func BenchmarkExpandURL(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := New(nil)
	defer exp.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exp.Expand(ctx, server.URL)
	}
}

func BenchmarkExpandBatch(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	exp := New(nil)
	defer exp.Close()

	urls := make([]string, 10)
	for i := range urls {
		urls[i] = server.URL
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exp.ExpandBatch(ctx, urls, 5)
	}
}
