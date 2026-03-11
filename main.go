package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Feed represents an RSS feed
type Feed struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Article represents a single RSS article
type Article struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Description string `json:"description"`
	Date        string `json:"date"`
}

// FeedData represents cached data for a feed
type FeedData struct {
	Articles   []Article `json:"articles"`
	LastUpdate string    `json:"last_update"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
}

// Cache represents the entire cache
type Cache struct {
	LastFetch string                `json:"last_fetch"`
	Feeds     map[string]FeedData   `json:"feeds"`
}

// RSS XML structures
type RSS struct {
	XMLName xml.Name  `xml:"rss"`
	Channel Channel   `xml:"channel"`
}

type Channel struct {
	Title       string    `xml:"title"`
	Items       []Item    `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// Default feeds
var feeds = []Feed{
	{Name: "Sud Ouest Bordeaux", URL: "http://www.sudouest.fr/gironde/bordeaux/rss.xml"},
	{Name: "Courrier International", URL: "http://www.courrierinternational.com/rss/all/rss.xml"},
	{Name: "YoanDev", URL: "https://flux.yoandev.co/rss.xml"},
	{Name: "France Info Sciences", URL: "http://www.francetvinfo.fr/sciences.rss"},
	{Name: "Hacker News", URL: "https://hnrss.org/frontpage"},
	{Name: "Le Monde IA", URL: "http://www.lemonde.fr/intelligence-artificielle/rss_full.xml"},
	{Name: "Korben", URL: "http://feeds.feedburner.com/KorbensBlog-UpgradeYourMind"},
	{Name: "Rue89 Bordeaux", URL: "http://feeds.feedburner.com/Rue89Bordeaux"},
	{Name: "Le Monde Sciences", URL: "http://www.lemonde.fr/rss/tag/sciences.xml"},
	{Name: "AWS Blog", URL: "http://blogs.aws.amazon.com/application-management/blog/feed/recentPosts.rss"},
	{Name: "Le Figaro Sciences", URL: "http://www.lefigaro.fr/rss/figaro_sciences.xml"},
}

const cacheFile = "cache.json"

func main() {
	// Initial cache update
	updateCache()

	// Start background updater
	go backgroundUpdater()

	// Setup routes
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)
	http.HandleFunc("/api/feeds", handleFeeds)
	http.HandleFunc("/api/cache", handleCache)

	port := "7007"
	fmt.Printf("🌐 Server running at http://localhost:%s\n", port)
	fmt.Println("Press Ctrl+C to stop")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleFeeds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	response := map[string]interface{}{
		"feeds": feeds,
		"count": len(feeds),
	}

	json.NewEncoder(w).Encode(response)
}

func handleCache(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	cache, err := loadCache()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Cache not ready",
			"feeds": map[string]FeedData{},
		})
		return
	}

	json.NewEncoder(w).Encode(cache)
}

func backgroundUpdater() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("🔄 Updating cache...")
		updateCache()
		log.Println("✅ Cache updated")
	}
}

func updateCache() {
	cache := Cache{
		LastFetch: time.Now().Format(time.RFC3339),
		Feeds:     make(map[string]FeedData),
	}

	for _, feed := range feeds {
		log.Printf("Fetching: %s...\n", feed.Name)
		start := time.Now()

		articles, err := fetchFeed(feed.URL)
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("  ✗ Error: %v (%.1fs)\n", err, elapsed.Seconds())
			cache.Feeds[feed.URL] = FeedData{
				Articles:   []Article{},
				LastUpdate: time.Now().Format(time.RFC3339),
				Status:     "error",
				Error:      err.Error(),
			}
		} else {
			log.Printf("  ✓ %d articles (%.1fs)\n", len(articles), elapsed.Seconds())
			cache.Feeds[feed.URL] = FeedData{
				Articles:   articles,
				LastUpdate: time.Now().Format(time.RFC3339),
				Status:     "ok",
			}
		}
	}

	saveCache(cache)
}

func fetchFeed(url string) ([]Article, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RSS Reader)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if it's Atom format
	bodyStr := string(body)
	if strings.Contains(bodyStr, "<feed") {
		return parseAtom(body)
	}

	return parseRSS(body)
}

func parseRSS(data []byte) ([]Article, error) {
	var rss RSS
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}

	var articles []Article
	for i, item := range rss.Channel.Items {
		if i >= 20 {
			break
		}
		articles = append(articles, Article{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Date:        item.PubDate,
		})
	}

	return articles, nil
}

func parseAtom(data []byte) ([]Article, error) {
	// Simple Atom parsing - extract entry elements
	// For a production app, use proper Atom struct
	var articles []Article

	// Look for entry patterns
	entries := strings.Split(string(data), "<entry")
	for i, entry := range entries {
		if i == 0 {
			continue // Skip header
		}
		if i > 20 {
			break
		}

		title := extractTag(entry, "title")
		link := extractAtomLink(entry)
		summary := extractTag(entry, "summary")
		if summary == "" {
			summary = extractTag(entry, "content")
		}
		updated := extractTag(entry, "updated")
		if updated == "" {
			updated = extractTag(entry, "published")
		}

		articles = append(articles, Article{
			Title:       title,
			Link:        link,
			Description: summary,
			Date:        updated,
		})
	}

	return articles, nil
}

func extractTag(xml, tag string) string {
	start := strings.Index(xml, "<"+tag)
	if start == -1 {
		return ""
	}
	start = strings.Index(xml[start:], ">") + start + 1
	end := strings.Index(xml[start:], "</"+tag+">")
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(xml[start : start+end])
}

func extractAtomLink(xml string) string {
	// Look for <link href="...">
	start := strings.Index(xml, `<link`)
	if start == -1 {
		return ""
	}
	hrefStart := strings.Index(xml[start:], `href="`)
	if hrefStart == -1 {
		return ""
	}
	hrefStart += start + 6
	hrefEnd := strings.Index(xml[hrefStart:], `"`)
	if hrefEnd == -1 {
		return ""
	}
	return xml[hrefStart : hrefStart+hrefEnd]
}

func loadCache() (Cache, error) {
	var cache Cache

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return cache, err
	}

	if err := json.Unmarshal(data, &cache); err != nil {
		return cache, err
	}

	return cache, nil
}

func saveCache(cache Cache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, data, 0644)
}
