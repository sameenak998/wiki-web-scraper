# Wikipedia Scraper

A Go program using the Colly web scraping framework to search Wikipedia for articles containing specific keywords in their titles, then automatically scrape and extract clean content from all matching articles. 

This scraper is designed for researchers who need to gather structured data from Wikipedia for text analysis, machine learning, or knowledge base construction. It searches for articles related to topics like artificial intelligence, robotics, machine learning, and more discovering and scraping multiple related articles per keyword rather than being limited to predefined URLs. The tool outputs clean, machine-readable data in NDJSON format, making it easy to integrate into data pipelines.

## Quick Start

```bash
go mod tidy
go run main.go
```

Results are saved to `wikipedia_scrape_results.ndjson` with fields: `url`, `title`, `text`.

## Features

- Searches Wikipedia API for articles with keywords in titles
- Scrapes multiple articles per keyword (up to 10 by default)
- Polite scraping with 2-second delays and rate limiting
- Removes duplicates, HTML markup, and navigation elements
- Outputs NDJSON format (one JSON object per line)

## Keywords Searched in this Example

- Artificial Intelligence
- Robotics  
- Machine Learning
- Computer Vision
- Autonomous Robot
- Intelligent Agent

## Output Format

Each line in `wikipedia_scrape_results.ndjson` is valid JSON:

```json
{"url":"https://en.wikipedia.org/wiki/Artificial_intelligence","title":"Artificial intelligence","text":"..."}
```

## Configuration

Edit `main()` to customize:
- `keywords` slice: Change search terms
- `SearchAndScrapeKeyword(keyword, 10)`: Adjust results per keyword
- `Delay` in `colly.LimitRule`: Adjust request delay (default 2 seconds)
- `outputFile`: Change output filename

## How It Works

1. **Search**: Query Wikipedia API for each keyword
2. **Collect**: Gather all found article URLs (filter duplicates)
3. **Scrape**: Visit each URL and extract clean text
4. **Save**: Write results as NDJSON

## Go vs. Python/Scrapy

This scraper provides similar functionality to a Python Scrapy crawler but with performance advantages. While Scrapy is more feature-rich for complex crawling scenarios, this Go implementation using Colly is:
- **Faster**: Go's compiled nature and efficient concurrency model typically execute 2-5x faster than Python
- **Simpler**: Lighter dependencies and smaller binary footprint (single executable vs. Python environment setup)
- **Self-contained**: No virtual environment or pip management needed—just run the compiled binary anywhere
- **Ideal for**: Quick scraping jobs, data collection pipelines, and resource-constrained environments

For comparison, a similar Python/Scrapy project would require virtualenv setup, multiple dependencies, and longer execution times, especially on large datasets.

## Requirements

- Go 1.21+
- `github.com/gocolly/colly/v2`

## GenAI Tools

This project was built with AI assistance using GitHub Copilot and Claude, which were used to design the architecture, implement the Colly integration, and debug compilation errors. AI tools accelerated development from requirements to a fully functional, tested scraper.
