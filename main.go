package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// ScrapedPage represents a single scraped Wikipedia page
type ScrapedPage struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// WikiScraper handles the scraping logic
type WikiScraper struct {
	collector   *colly.Collector
	visitedURLs map[string]bool
	results     []*ScrapedPage
	outputFile  *os.File
}

// NewWikiScraper initializes a new Wikipedia scraper
func NewWikiScraper(outputFilename string) (*WikiScraper, error) {
	outputFile, err := os.Create(outputFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}

	// Create a new collector
	c := colly.NewCollector(
		colly.AllowedDomains("en.wikipedia.org", "wikipedia.org", "*.wikipedia.org"),
		colly.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"),
	)

	// Set rate limiting - polite scraping with delays
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*wikipedia.org*",
		Parallelism: 1,
		Delay:       2 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	// Allow revisits to the same URL
	c.AllowURLRevisit = true

	ws := &WikiScraper{
		collector:   c,
		visitedURLs: make(map[string]bool),
		results:     make([]*ScrapedPage, 0),
		outputFile:  outputFile,
	}

	// Setup callbacks
	ws.setupCallbacks()

	return ws, nil
}

// setupCallbacks configures the collector's callbacks
func (ws *WikiScraper) setupCallbacks() {
	// Log request
	ws.collector.OnRequest(func(r *colly.Request) {
		log.Printf("[REQUEST] Visiting %s\n", r.URL)
	})

	// Track response
	ws.collector.OnResponse(func(r *colly.Response) {
		log.Printf("[RESPONSE] Got response from %s (Status: %d, Body size: %d bytes)\n", r.Request.URL, r.StatusCode, len(r.Body))
	})

	// Extract content after page loads
	ws.collector.OnHTML("body", func(r *colly.HTMLElement) {
		requestURL := r.Request.URL.String()
		log.Printf("[HTML] Processing body element from %s\n", requestURL)

		// Extract title
		title := strings.TrimSpace(r.ChildText("h1.firstHeading span.mw-page-title-main"))
		if title == "" {
			// Fallback to alternate title location
			title = strings.TrimSpace(r.ChildText("h1"))
		}

		// Extract main content text
		text := ws.extractMainText(r)

		log.Printf("[EXTRACT] Title: %q, Text length: %d\n", title, len(text))

		if title != "" && text != "" {
			page := &ScrapedPage{
				URL:   requestURL,
				Title: title,
				Text:  text,
			}

			// Write to NDJSON format immediately
			err := ws.writeNDJSON(page)
			if err != nil {
				log.Printf("[ERROR] Error writing to file: %v\n", err)
			} else {
				log.Printf("[SUCCESS] Scraped: %s\n", title)
				ws.results = append(ws.results, page)
			}
		} else {
			log.Printf("[SKIP] %s - Title: %q, Text length: %d\n", requestURL, title, len(text))
		}
	})

	// Handle errors
	ws.collector.OnError(func(r *colly.Response, err error) {
		if r != nil {
			log.Printf("[COLLY-ERROR] %d: %v for %s\n", r.StatusCode, err, r.Request.URL)
		} else {
			log.Printf("[COLLY-ERROR] %v\n", err)
		}
	})
}

// extractMainText extracts clean readable text from Wikipedia page content
func (ws *WikiScraper) extractMainText(r *colly.HTMLElement) string {
	var textParts []string

	// Extract paragraphs from the main content area using ForEach
	r.ForEach("div#mw-content-main p", func(_ int, el *colly.HTMLElement) {
		text := strings.TrimSpace(el.Text)
		if text != "" && !strings.Contains(text, "href") { // Filter out pure link text
			textParts = append(textParts, text)
		}
	})

	// If no paragraphs found, try alternate selector
	if len(textParts) == 0 {
		r.ForEach("div.mw-parser-output p", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if text != "" {
				textParts = append(textParts, text)
			}
		})
	}

	// Join paragraphs with newlines
	fullText := strings.Join(textParts, "\n")

	// If still empty, get first 3000 chars of mw-content-main as fallback
	if fullText == "" {
		fullText = strings.TrimSpace(r.ChildText("div#mw-content-main"))
		if len(fullText) > 3000 {
			fullText = fullText[:3000]
		}
	}

	// Clean up the text
	fullText = ws.cleanText(fullText)

	return fullText

}

// cleanText removes unwanted characters and normalizes whitespace
func (ws *WikiScraper) cleanText(text string) string {
	// Remove reference brackets like [1], [2], etc.
	refRE := regexp.MustCompile(`\[\d+\]`)
	text = refRE.ReplaceAllString(text, "")

	// Remove citation needed markers
	text = strings.ReplaceAll(text, "[citation needed]", "")

	// Remove multiple spaces
	spaceRE := regexp.MustCompile(`\s+`)
	text = spaceRE.ReplaceAllString(text, " ")

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	// Remove lines that are just whitespace
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// writeNDJSON writes a page to the output file in NDJSON format
func (ws *WikiScraper) writeNDJSON(page *ScrapedPage) error {
	jsonBytes, err := json.Marshal(page)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	_, err = ws.outputFile.WriteString(string(jsonBytes) + "\n")
	if err != nil {
		return fmt.Errorf("failed to write to output file: %w", err)
	}

	return nil
}

// Crawl starts the scraping process from a list of starting URLs
func (ws *WikiScraper) Crawl(startingURLs []string) error {
	log.Printf("[CRAWL] Starting to crawl %d URLs\n", len(startingURLs))

	for i, startURL := range startingURLs {
		if _, visited := ws.visitedURLs[startURL]; visited {
			log.Printf("[CRAWL] URL %d already visited: %s\n", i+1, startURL)
			continue
		}

		ws.visitedURLs[startURL] = true
		log.Printf("[CRAWL] About to visit URL %d/%d: %s\n", i+1, len(startingURLs), startURL)

		err := ws.collector.Visit(startURL)
		if err != nil {
			log.Printf("[CRAWL] Error visiting %s: %v\n", startURL, err)
			continue
		}
		log.Printf("[CRAWL] Successfully completed visit to: %s\n", startURL)
	}

	return nil
}

// isValidWikipediaURL checks if a URL is within Wikipedia scope
func (ws *WikiScraper) isValidWikipediaURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Only allow en.wikipedia.org and wikipedia.org
	domain := parsedURL.Host
	if !strings.Contains(domain, "wikipedia.org") {
		return false
	}

	// Avoid special pages, talk pages, and non-article pages
	path := parsedURL.Path
	if strings.Contains(path, "Special:") ||
		strings.Contains(path, "Talk:") ||
		strings.Contains(path, "Wikipedia:") ||
		strings.Contains(path, "File:") ||
		strings.Contains(path, "Template:") {
		return false
	}

	return true
}

// Close closes the output file
func (ws *WikiScraper) Close() error {
	return ws.outputFile.Close()
}

// GetResults returns the scraped results
func (ws *WikiScraper) GetResults() []*ScrapedPage {
	return ws.results
}

// SearchAndScrapeKeyword searches for Wikipedia pages with keyword in title and scrapes them
func (ws *WikiScraper) SearchAndScrapeKeyword(keyword string, maxResults int) error {
	log.Printf("[SEARCH] Searching for pages with keyword: %q\n", keyword)

	// Create search URL
	searchURL := fmt.Sprintf("https://en.wikipedia.org/w/api.php?action=query&list=search&srsearch=intitle:%s&srwhat=text&format=json&srlimit=%d",
		url.QueryEscape(keyword), maxResults)

	log.Printf("[SEARCH] Fetching: %s\n", searchURL)

	// Fetch search results via JSON API
	searchCollector := colly.NewCollector()
	searchCollector.OnRequest(func(r *colly.Request) {
		log.Printf("[SEARCH-REQUEST] Getting search results for: %s\n", keyword)
	})

	var searchResults map[string]interface{}
	searchCollector.OnResponse(func(r *colly.Response) {
		err := json.Unmarshal(r.Body, &searchResults)
		if err != nil {
			log.Printf("[SEARCH-ERROR] Failed to parse JSON: %v\n", err)
			return
		}

		// Extract article titles from search results
		if query, ok := searchResults["query"].(map[string]interface{}); ok {
			if search, ok := query["search"].([]interface{}); ok {
				log.Printf("[SEARCH-RESULTS] Found %d results for keyword %q\n", len(search), keyword)

				for _, result := range search {
					if resultMap, ok := result.(map[string]interface{}); ok {
						if title, ok := resultMap["title"].(string); ok {
							// Construct Wikipedia article URL from title
							articleURL := fmt.Sprintf("https://en.wikipedia.org/wiki/%s",
								url.PathEscape(strings.ReplaceAll(title, " ", "_")))

							log.Printf("[SEARCH-FOUND] Article: %s -> %s\n", title, articleURL)

							// Mark as to-visit and will be scraped
							if _, visited := ws.visitedURLs[articleURL]; !visited {
								ws.visitedURLs[articleURL] = false // Mark as found, not visited yet
							}
						}
					}
				}
			}
		}
	})

	searchCollector.OnError(func(_ *colly.Response, err error) {
		log.Printf("[SEARCH-ERROR] %v\n", err)
	})

	err := searchCollector.Visit(searchURL)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	return nil
}

// CrawlFoundArticles crawls all articles marked as found but not yet visited
func (ws *WikiScraper) CrawlFoundArticles() error {
	log.Println("[CRAWL] Starting to crawl found articles")
	crawledCount := 0

	for articleURL, visited := range ws.visitedURLs {
		if !visited && strings.Contains(articleURL, "/wiki/") {
			ws.visitedURLs[articleURL] = true
			log.Printf("[CRAWL] Visiting found article: %s\n", articleURL)

			err := ws.collector.Visit(articleURL)
			if err != nil {
				log.Printf("[CRAWL-ERROR] Error visiting %s: %v\n", articleURL, err)
				continue
			}
			crawledCount++
		}
	}

	log.Printf("[CRAWL] Crawled %d found articles\n", crawledCount)
	return nil
}

func main() {
	// Output file
	outputFile := "wikipedia_scrape_results.ndjson"

	// Create scraper
	scraper, err := NewWikiScraper(outputFile)
	if err != nil {
		log.Fatalf("Failed to create scraper: %v", err)
	}
	defer scraper.Close()

	// Keywords to search for in Wikipedia article titles
	keywords := []string{
		"Artificial intelligence",
		"Robotics",
		"Machine learning",
		"Computer vision",
		"Autonomous robot",
		"Intelligent agent",
	}

	log.Println("Starting Wikipedia scraper with keyword search...")
	log.Printf("Target output file: %s\n", outputFile)
	log.Printf("Keywords to search: %d\n", len(keywords))

	// Search for each keyword and collect article URLs
	for _, keyword := range keywords {
		err := scraper.SearchAndScrapeKeyword(keyword, 10) // Search for up to 10 results per keyword
		if err != nil {
			log.Printf("Warning: Search for %q failed: %v\n", keyword, err)
		}
		time.Sleep(1 * time.Second) // Be polite between searches
	}

	log.Printf("[MAIN] Found %d unique articles to scrape\n", len(scraper.visitedURLs))

	// Now crawl all the found articles
	err = scraper.CrawlFoundArticles()
	if err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	log.Printf("Scraping completed. Results saved to %s\n", outputFile)
	log.Printf("Total pages scraped: %d\n", len(scraper.GetResults()))

	// Print summary
	for i, result := range scraper.GetResults() {
		log.Printf("Page %d: %s (%d characters)\n", i+1, result.Title, len(result.Text))
	}
}
