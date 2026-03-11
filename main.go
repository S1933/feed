package main

import (
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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
	LastFetch string              `json:"last_fetch"`
	Feeds     map[string]FeedData `json:"feeds"`
}

// RSS XML structures
type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title string `xml:"title"`
	Items []Item `xml:"item"`
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
	{Name: "Kilo AI Blog", URL: "https://blog.kilo.ai/feed"},
}

const cacheFile = "cache.json"
const dbFile = "rss.db"

var db *sql.DB
var refreshMutex sync.Mutex
var refreshInProgress bool

func main() {
	// Initialize database
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initial cache update
	updateCache()

	// Start background updater
	go backgroundUpdater()

	// Setup routes - only serve specific frontend files
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/app.js", handleStaticFile("app.js", "application/javascript"))
	http.HandleFunc("/styles.css", handleStaticFile("styles.css", "text/css"))
	http.HandleFunc("/api/feeds", handleFeeds)
	http.HandleFunc("/api/cache", handleCache)
	http.HandleFunc("/api/refresh", handleRefresh)
	http.HandleFunc("/api/read", handleReadArticles)
	http.HandleFunc("/api/favorites", handleFavorites)

	port := "7007"
	fmt.Printf("🌐 Server running at http://127.0.0.1:%s\n", port)
	fmt.Println("Press Ctrl+C to stop")
	log.Fatal(http.ListenAndServe("127.0.0.1:"+port, nil))
}

// handleIndex serves the index.html file
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

// handleStaticFile returns a handler for static files
func handleStaticFile(filename, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", contentType)
		http.ServeFile(w, r, filename)
	}
}

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}

	// Create tables
	createTables := `
	CREATE TABLE IF NOT EXISTS read_articles (
		article_id TEXT PRIMARY KEY,
		read_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS favorite_articles (
		article_id TEXT PRIMARY KEY,
		favorited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTables)
	return err
}

func handleFeeds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"feeds": feeds,
		"count": len(feeds),
	}

	json.NewEncoder(w).Encode(response)
}

func handleCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

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

func handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	// Try to acquire lock
	refreshMutex.Lock()
	if refreshInProgress {
		refreshMutex.Unlock()
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Refresh already in progress",
		})
		return
	}
	refreshInProgress = true
	refreshMutex.Unlock()

	// Run cache update in background to not block the response
	go func() {
		defer func() {
			refreshMutex.Lock()
			refreshInProgress = false
			refreshMutex.Unlock()
		}()
		log.Println("🔄 Manual cache refresh triggered...")
		if err := updateCacheWithError(); err != nil {
			log.Printf("✗ Manual cache refresh failed: %v\n", err)
		} else {
			log.Println("✅ Manual cache refresh completed")
		}
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Cache refresh triggered",
	})
}

func handleReadArticles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// Get all read articles
		rows, err := db.Query("SELECT article_id, read_at FROM read_articles")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer rows.Close()

		readArticles := make(map[string]string)
		for rows.Next() {
			var id string
			var readAt string
			if err := rows.Scan(&id, &readAt); err == nil {
				readArticles[id] = readAt
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"read_articles": readArticles,
		})

	case "POST":
		// Mark article as read
		var req struct {
			ArticleID string `json:"article_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request"})
			return
		}

		_, err := db.Exec("INSERT OR REPLACE INTO read_articles (article_id) VALUES (?)", req.ArticleID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"article_id": req.ArticleID,
		})

	case "DELETE":
		// Clear all read articles (optional)
		_, err := db.Exec("DELETE FROM read_articles")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}
}

func handleFavorites(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		// Get all favorites
		rows, err := db.Query("SELECT article_id, favorited_at FROM favorite_articles")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}
		defer rows.Close()

		favorites := make(map[string]string)
		for rows.Next() {
			var id string
			var favAt string
			if err := rows.Scan(&id, &favAt); err == nil {
				favorites[id] = favAt
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"favorites": favorites,
		})

	case "POST":
		// Toggle favorite
		var req struct {
			ArticleID string `json:"article_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "Invalid request"})
			return
		}

		// Check if already favorited
		var exists int
		err := db.QueryRow("SELECT COUNT(*) FROM favorite_articles WHERE article_id = ?", req.ArticleID).Scan(&exists)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		if exists > 0 {
			// Remove from favorites
			_, err = db.Exec("DELETE FROM favorite_articles WHERE article_id = ?", req.ArticleID)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"article_id": req.ArticleID,
				"is_favorite": false,
			})
		} else {
			// Add to favorites
			_, err = db.Exec("INSERT INTO favorite_articles (article_id) VALUES (?)", req.ArticleID)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"article_id": req.ArticleID,
				"is_favorite": true,
			})
		}

	case "DELETE":
		// Clear all favorites (optional)
		_, err := db.Exec("DELETE FROM favorite_articles")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
		})
	}
}

func backgroundUpdater() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		refreshMutex.Lock()
		if refreshInProgress {
			refreshMutex.Unlock()
			log.Println("⏳ Skipping scheduled refresh - another refresh is in progress")
			continue
		}
		refreshInProgress = true
		refreshMutex.Unlock()

		log.Println("🔄 Updating cache...")
		if err := updateCacheWithError(); err != nil {
			log.Printf("✗ Cache update failed: %v\n", err)
		} else {
			log.Println("✅ Cache updated")
		}

		refreshMutex.Lock()
		refreshInProgress = false
		refreshMutex.Unlock()
	}
}

// updateCache calls updateCacheWithError and logs errors
func updateCache() {
	if err := updateCacheWithError(); err != nil {
		log.Printf("Cache update error: %v\n", err)
	}
}

// updateCacheWithError performs the actual cache update and returns any error
func updateCacheWithError() error {
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

	return saveCache(cache)
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

	// Create temp file in same directory for atomic write
	dir := filepath.Dir(cacheFile)
	if dir == "." {
		dir = ""
	}
	tempFile := filepath.Join(dir, ".cache.json.tmp")

	// Write to temp file
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp cache file: %w", err)
	}

	// Open temp file for fsync
	f, err := os.Open(tempFile)
	if err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to open temp cache file for fsync: %w", err)
	}

	// Fsync to ensure data is written to disk
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tempFile)
		return fmt.Errorf("failed to fsync cache file: %w", err)
	}
	f.Close()

	// Atomic rename
	if err := os.Rename(tempFile, cacheFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename cache file: %w", err)
	}

	return nil
}
