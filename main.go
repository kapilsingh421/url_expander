package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kapilsingh421/url-expander/expander"
)

const separator = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

func main() {
	// Command line flags
	urlFlag := flag.String("url", "", "Single URL to expand")
	urlsFlag := flag.String("urls", "", "Comma-separated list of URLs to expand")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "Request timeout")
	maxRedirectsFlag := flag.Int("max-redirects", 20, "Maximum number of redirects to follow")
	concurrencyFlag := flag.Int("concurrency", 5, "Number of concurrent requests for batch expansion")
	jsonFlag := flag.Bool("json", false, "Output results as JSON")
	verboseFlag := flag.Bool("verbose", false, "Show detailed redirect chain")
	testFlag := flag.Bool("test", false, "Run built-in tests with known short URLs")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `URL Expander - Production-grade short URL expansion tool

Usage:
  %s [options]

Options:
`, os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Expand a single URL
  %s -url "https://t.co/dKP3o1e"

  # Expand multiple URLs
  %s -urls "https://t.co/dKP3o1e,https://bit.ly/example"

  # Output as JSON
  %s -url "https://t.co/dKP3o1e" -json

  # Show redirect chain
  %s -url "https://t.co/dKP3o1e" -verbose

  # Run built-in tests
  %s -test
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	}

	flag.Parse()

	// Create expander with configuration
	config := expander.DefaultConfig()
	config.Timeout = *timeoutFlag
	config.MaxRedirects = *maxRedirectsFlag

	exp := expander.New(config)
	defer exp.Close()

	ctx := context.Background()

	// Handle different modes
	switch {
	case *testFlag:
		runTests(ctx, exp, *verboseFlag, *jsonFlag)
	case *urlFlag != "":
		expandSingle(ctx, exp, *urlFlag, *verboseFlag, *jsonFlag)
	case *urlsFlag != "":
		urls := strings.Split(*urlsFlag, ",")
		for i := range urls {
			urls[i] = strings.TrimSpace(urls[i])
		}
		expandBatch(ctx, exp, urls, *concurrencyFlag, *verboseFlag, *jsonFlag)
	default:
		// If no flags, run interactive mode or show help
		if len(os.Args) == 1 {
			runTests(ctx, exp, true, false)
		} else {
			flag.Usage()
		}
	}
}

func expandSingle(ctx context.Context, exp *expander.Expander, url string, verbose, jsonOutput bool) {
	result := exp.Expand(ctx, url)

	if jsonOutput {
		outputJSON(result)
		return
	}

	printResult(result, verbose)
}

func expandBatch(ctx context.Context, exp *expander.Expander, urls []string, concurrency int, verbose, jsonOutput bool) {
	results := exp.ExpandBatch(ctx, urls, concurrency)

	if jsonOutput {
		output := make([]map[string]interface{}, len(results))
		for i, r := range results {
			output[i] = resultToMap(r)
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		fmt.Println(string(data))
		return
	}

	for _, result := range results {
		printResult(result, verbose)
		fmt.Println()
	}
}

func printResult(result *expander.ExpansionResult, verbose bool) {
	fmt.Printf("Original:  %s\n", result.OriginalURL)

	if verbose && len(result.RedirectChain) > 0 {
		fmt.Println("Redirects:")
		for i, r := range result.RedirectChain {
			fmt.Printf("  [%d] %d %s\n", i+1, r.StatusCode, r.ToURL)
			fmt.Printf("      └── %s\n", r.Duration)
		}
	}

	if result.Error != nil {
		fmt.Printf("Error:     %v\n", result.Error)
	}

	fmt.Printf("Expanded:  %s\n", result.FinalURL)
	fmt.Printf("Hops:      %d\n", result.Hops)
	fmt.Printf("Duration:  %s\n", result.TotalTime)
}

func outputJSON(result *expander.ExpansionResult) {
	data, _ := json.MarshalIndent(resultToMap(result), "", "  ")
	fmt.Println(string(data))
}

func resultToMap(result *expander.ExpansionResult) map[string]interface{} {
	m := map[string]interface{}{
		"original_url": result.OriginalURL,
		"expanded_url": result.FinalURL,
		"hops":         result.Hops,
		"duration_ms":  result.TotalTime.Milliseconds(),
	}

	if result.Error != nil {
		m["error"] = result.Error.Error()
	}

	if len(result.RedirectChain) > 0 {
		chain := make([]map[string]interface{}, len(result.RedirectChain))
		for i, r := range result.RedirectChain {
			chain[i] = map[string]interface{}{
				"from":        r.FromURL,
				"to":          r.ToURL,
				"status_code": r.StatusCode,
				"duration_ms": r.Duration.Milliseconds(),
			}
		}
		m["redirect_chain"] = chain
	}

	return m
}

func runTests(ctx context.Context, exp *expander.Expander, verbose, jsonOutput bool) {
	printHeader()
	runIndividualTests(ctx, exp, verbose)
	runBatchDemo(ctx, exp)
	printUsageExamples()
}

func printHeader() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║   URL Expander - Production Grade Short URL Expansion Tool   ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func runIndividualTests(ctx context.Context, exp *expander.Expander, verbose bool) {
	testURLs := []struct {
		name string
		url  string
	}{
		{"Twitter (t.co)", "https://t.co/dKP3o1e"},
		{"HTTPBin Redirect", "https://httpbin.org/redirect-to?url=https://www.google.com"},
		{"HTTPBin Multi-hop", "https://httpbin.org/absolute-redirect/3"},
		{"TinyURL", "https://tinyurl.com/yc68mvwd"},
		{"Bitly", "https://bit.ly/3xyz123"},
	}

	fmt.Println("Running expansion tests...")
	fmt.Println(separator)
	fmt.Println()

	for _, test := range testURLs {
		result := exp.Expand(ctx, test.url)
		printTestResult(test.name, test.url, result, verbose)
	}

	fmt.Println(separator)
	fmt.Println()
}

func printTestResult(name, url string, result *expander.ExpansionResult, verbose bool) {
	fmt.Printf(" %s\n", name)
	fmt.Printf("  Input: %s\n", url)

	if verbose && len(result.RedirectChain) > 0 {
		printRedirectChain(result.RedirectChain)
	}

	if result.Error != nil {
		fmt.Printf("   Error: %v\n", result.Error)
	}

	status := getStatusIcon(result)
	fmt.Printf("  %s Expanded: %s\n", status, truncateURL(result.FinalURL, 60))
	fmt.Printf("   Duration: %s | Hops: %d\n", result.TotalTime, result.Hops)
	fmt.Println()
}

func printRedirectChain(chain []expander.RedirectInfo) {
	fmt.Println("  Redirect Chain:")
	for i, r := range chain {
		fmt.Printf("    [%d] %d → %s\n", i+1, r.StatusCode, truncateURL(r.ToURL, 60))
	}
}

func getStatusIcon(result *expander.ExpansionResult) string {
	if result.Error != nil && result.FinalURL == "" {
		return "✗"
	}
	return "✓"
}

func runBatchDemo(ctx context.Context, exp *expander.Expander) {
	fmt.Println(" Batch Expansion Demo (concurrent)")
	urls := []string{
		"https://t.co/dKP3o1e",
		"https://httpbin.org/redirect-to?url=https://example.com",
		"https://httpbin.org/absolute-redirect/2",
	}

	start := time.Now()
	results := exp.ExpandBatch(ctx, urls, 3)
	batchDuration := time.Since(start)

	for i, result := range results {
		status := getStatusIcon(result)
		fmt.Printf("  %s [%d] %s → %s\n", status, i+1, truncateURL(result.OriginalURL, 30), truncateURL(result.FinalURL, 40))
	}
	fmt.Printf("   Total batch time: %s\n", batchDuration)
	fmt.Println()
}

func printUsageExamples() {
	fmt.Println(separator)
	fmt.Println("Usage Examples:")
	fmt.Println("  go run main.go -url \"https://t.co/dKP3o1e\"")
	fmt.Println("  go run main.go -url \"https://bit.ly/xxx\" -verbose")
	fmt.Println("  go run main.go -url \"https://tinyurl.com/xxx\" -json")
	fmt.Println("  go run main.go -urls \"url1,url2,url3\" -concurrency 10")
	fmt.Println(separator)
}

func truncateURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	return url[:maxLen-3] + "..."
}
