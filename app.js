class FeedReader {
    constructor() {
        this.feeds = [];
        this.cache = null;
        this.allArticles = [];
        this.readArticles = this.loadReadArticles();
        this.favoriteArticles = this.loadFavoriteArticles();
        this.init();
    }

    // === DATABASE API ===

    async loadReadArticles() {
        try {
            const res = await fetch('/api/read');
            const data = await res.json();
            return data.read_articles || {};
        } catch (e) {
            console.error('Error loading read articles:', e);
            return {};
        }
    }

    async saveReadArticle(articleId) {
        try {
            await fetch('/api/read', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ article_id: articleId })
            });
        } catch (e) {
            console.error('Error saving read article:', e);
        }
    }

    async loadFavoriteArticles() {
        try {
            const res = await fetch('/api/favorites');
            const data = await res.json();
            return data.favorites || {};
        } catch (e) {
            console.error('Error loading favorites:', e);
            return {};
        }
    }

    async markAsRead(articleId) {
        this.readArticles[articleId] = new Date().toISOString();
        await this.saveReadArticle(articleId);
    }

    async markAllAsRead() {
        let articles = this.allArticles;

        if (this.currentFilter === 'favorites') {
            articles = articles.filter(a => this.isFavorite(this.getArticleId(a)));
        } else if (this.currentFilter !== 'all') {
            articles = articles.filter(a => a.feedUrl === this.currentFilter);
        }

        for (const article of articles) {
            const articleId = this.getArticleId(article);
            if (!this.readArticles[articleId]) {
                this.readArticles[articleId] = new Date().toISOString();
                await this.saveReadArticle(articleId);
            }
        }

        // Re-render to hide read articles
        this.renderAllArticles(this.currentFilter);
    }

    isRead(articleId) {
        return !!this.readArticles[articleId];
    }

    async toggleFavorite(articleId) {
        try {
            const res = await fetch('/api/favorites', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ article_id: articleId })
            });
            const data = await res.json();

            if (data.is_favorite) {
                this.favoriteArticles[articleId] = new Date().toISOString();
            } else {
                delete this.favoriteArticles[articleId];
            }

            return data.is_favorite;
        } catch (e) {
            console.error('Error toggling favorite:', e);
            return this.isFavorite(articleId);
        }
    }

    isFavorite(articleId) {
        return !!this.favoriteArticles[articleId];
    }

    // === INIT ===

    async init() {
        await this.loadFeeds();
        await this.loadCache();
        this.readArticles = await this.loadReadArticles();
        this.favoriteArticles = await this.loadFavoriteArticles();
        this.mergeAllArticles();
        this.renderFeedList();
        this.renderAllArticles();
        this.bindEvents();
    }

    // === API CALLS ===

    async loadFeeds() {
        try {
            const res = await fetch('/api/feeds');
            const data = await res.json();
            this.feeds = data.feeds || [];
        } catch (err) {
            console.error('Error loading feeds:', err);
            this.feeds = [];
        }
    }

    async loadCache() {
        try {
            const res = await fetch('/api/cache');
            this.cache = await res.json();
        } catch (err) {
            console.error('Error loading cache:', err);
            this.cache = { feeds: {} };
        }
    }

    // === MERGE & SORT ARTICLES ===

    mergeAllArticles() {
        this.allArticles = [];

        for (const feed of this.feeds) {
            const feedData = this.cache?.feeds?.[feed.url];
            if (feedData && feedData.status === 'ok' && feedData.articles) {
                for (const article of feedData.articles) {
                    this.allArticles.push({
                        ...article,
                        feedName: feed.name,
                        feedUrl: feed.url,
                        parsedDate: this.parseDate(article.date)
                    });
                }
            }
        }

        this.allArticles.sort((a, b) => b.parsedDate - a.parsedDate);
    }

    parseDate(dateStr) {
        if (!dateStr) return new Date(0);
        try {
            return new Date(dateStr);
        } catch {
            return new Date(0);
        }
    }

    // === UI FUNCTIONS ===

    renderFeedList() {
        const container = document.getElementById('feed-list');
        const favCount = Object.keys(this.favoriteArticles).length;

        container.innerHTML = `
            <button class="feed-btn ${this.currentFilter === 'all' ? 'active' : ''}" data-filter="all">
                📰 Tous (${this.allArticles.length})
            </button>
            <button class="feed-btn ${this.currentFilter === 'favorites' ? 'active' : ''}" data-filter="favorites">
                ⭐ Favoris (${favCount})
            </button>
            <div class="feed-divider"></div>
            ${this.feeds.map(feed => {
                const count = this.allArticles.filter(a => a.feedUrl === feed.url).length;
                return `
                    <button class="feed-btn ${this.currentFilter === feed.url ? 'active' : ''}" data-filter="${feed.url}">
                        ${feed.name} (${count})
                    </button>
                `;
            }).join('')}
        `;
    }

    renderAllArticles(filter = 'all') {
        this.currentFilter = filter;
        const container = document.getElementById('articles');

        let articles = this.allArticles;

        if (filter === 'favorites') {
            articles = articles.filter(a => this.isFavorite(this.getArticleId(a)));
        } else if (filter !== 'all') {
            articles = articles.filter(a => a.feedUrl === filter);
        }

        // Filter out read articles
        const unreadArticles = articles.filter(a => !this.isRead(this.getArticleId(a)));

        if (unreadArticles.length === 0) {
            container.innerHTML = `
                <p class="empty">Aucun article non lu 🎉</p>
            `;
            return;
        }

        container.innerHTML = `
            ${unreadArticles.map(a => this.createArticleHTML(a)).join('')}
            <div class="mark-all-read-container">
                <button class="mark-all-read-btn" title="Tout marquer comme lu">
                    <span class="btn-text-full">✓ Tout marquer comme lu</span>
                    <span class="btn-text-short">✓ Tout lu</span>
                </button>
            </div>
        `;
    }

    getArticleId(article) {
        return btoa(article.link || article.title).replace(/[^a-zA-Z0-9]/g, '').slice(0, 32);
    }

    createArticleHTML(article) {
        const articleId = this.getArticleId(article);
        const isRead = this.isRead(articleId);
        const isFav = this.isFavorite(articleId);
        const title = this.escapeHtml(article.title || 'Sans titre');
        const desc = this.escapeHtml(this.stripHtml(article.description || '').slice(0, 200));
        const date = article.date ? this.formatDate(article.date) : '';

        return `
            <article class="article ${isRead ? 'read' : ''}" data-id="${articleId}">
                <div class="article-header">
                    <span class="site-badge">${this.escapeHtml(article.feedName)}</span>
                    <button class="fav-btn ${isFav ? 'active' : ''}" data-id="${articleId}" title="Favori">
                        ${isFav ? '⭐' : '☆'}
                    </button>
                </div>
                <a href="${article.link || '#'}" target="_blank" rel="noopener" data-id="${articleId}">
                    <h3>${title}</h3>
                    ${date ? `<time>${date}</time>` : ''}
                    <p>${desc}...</p>
                </a>
            </article>
        `;
    }

    // === EVENTS ===

    bindEvents() {
        // Filter by feed
        document.getElementById('feed-list').addEventListener('click', (e) => {
            const btn = e.target.closest('.feed-btn');
            if (!btn) return;

            this.renderAllArticles(btn.dataset.filter);
            this.renderFeedList(); // Re-render to update active state
        });

        // Mark as read on click
        document.getElementById('articles').addEventListener('click', async (e) => {
            const link = e.target.closest('a');
            if (link) {
                const articleId = link.dataset.id;
                if (articleId) {
                    await this.markAsRead(articleId);
                    // Re-render to hide the read article
                    setTimeout(() => this.renderAllArticles(this.currentFilter), 100);
                }
            }

            // Toggle favorite
            const favBtn = e.target.closest('.fav-btn');
            if (favBtn) {
                e.preventDefault();
                e.stopPropagation();
                const articleId = favBtn.dataset.id;
                const isNowFav = await this.toggleFavorite(articleId);
                favBtn.innerHTML = isNowFav ? '⭐' : '☆';
                favBtn.classList.toggle('active', isNowFav);
                this.renderFeedList(); // Update fav count
            }

            // Mark all as read
            const markAllBtn = e.target.closest('.mark-all-read-btn');
            if (markAllBtn) {
                e.preventDefault();
                e.stopPropagation();
                await this.markAllAsRead();
            }
        });

        // Refresh button
        const refreshBtn = document.getElementById('refresh-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => this.refreshFeeds());
        }
    }

    async refreshFeeds() {
        const refreshBtn = document.getElementById('refresh-btn');

        try {
            // Add spinning animation
            refreshBtn.classList.add('spinning');
            refreshBtn.disabled = true;

            // Call refresh API
            const res = await fetch('/api/refresh', { method: 'POST' });
            const data = await res.json();

            if (data.success) {
                // Wait a bit for the server to start fetching, then poll for updates
                setTimeout(async () => {
                    await this.pollForUpdates();
                }, 2000);
            } else {
                throw new Error(data.message || 'Refresh failed');
            }
        } catch (err) {
            console.error('Error refreshing feeds:', err);
            refreshBtn.classList.remove('spinning');
            refreshBtn.disabled = false;
            alert('Erreur lors du rafraîchissement des flux');
        }
    }

    async pollForUpdates() {
        const refreshBtn = document.getElementById('refresh-btn');
        let attempts = 0;
        const maxAttempts = 30; // Max 30 attempts (30 seconds)

        const checkInterval = setInterval(async () => {
            attempts++;

            try {
                // Reload cache from server
                await this.loadCache();

                // Check if cache has been updated recently (within last 2 minutes)
                if (this.cache?.last_fetch) {
                    const lastFetch = new Date(this.cache.last_fetch);
                    const now = new Date();
                    const diffMinutes = (now - lastFetch) / 1000 / 60;

                    if (diffMinutes < 2) {
                        // Cache was updated!
                        clearInterval(checkInterval);
                        this.mergeAllArticles();
                        this.renderFeedList();
                        this.renderAllArticles(this.currentFilter);
                        refreshBtn.classList.remove('spinning');
                        refreshBtn.disabled = false;
                        return;
                    }
                }

                if (attempts >= maxAttempts) {
                    clearInterval(checkInterval);
                    refreshBtn.classList.remove('spinning');
                    refreshBtn.disabled = false;
                    console.log('Refresh timeout - server still processing');
                }
            } catch (err) {
                console.error('Error checking for updates:', err);
                if (attempts >= maxAttempts) {
                    clearInterval(checkInterval);
                    refreshBtn.classList.remove('spinning');
                    refreshBtn.disabled = false;
                }
            }
        }, 1000);
    }

    updateUnreadCount() {
        let articles = this.allArticles;
        if (this.currentFilter === 'favorites') {
            articles = articles.filter(a => this.isFavorite(this.getArticleId(a)));
        } else if (this.currentFilter !== 'all') {
            articles = articles.filter(a => a.feedUrl === this.currentFilter);
        }

        const unreadCount = articles.filter(a => !this.isRead(this.getArticleId(a))).length;

        const statusBar = document.querySelector('.status-bar span:first-child');
        if (statusBar) {
            statusBar.textContent = `${unreadCount}/${articles.length} non lus`;
        }
    }

    // === UTILS ===

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text || '';
        return div.innerHTML;
    }

    stripHtml(html) {
        const div = document.createElement('div');
        div.innerHTML = html || '';
        return div.textContent || '';
    }

    formatDate(dateStr) {
        try {
            const d = new Date(dateStr);
            const now = new Date();
            const diff = (now - d) / 1000;

            if (diff < 60) return "à l'instant";
            if (diff < 3600) return `il y a ${Math.floor(diff / 60)} min`;
            if (diff < 86400) return `il y a ${Math.floor(diff / 3600)}h`;
            if (diff < 604800) return `il y a ${Math.floor(diff / 86400)}j`;

            return d.toLocaleDateString('fr-FR', {
                day: 'numeric', month: 'short'
            });
        } catch {
            return dateStr;
        }
    }
}

// Start
document.addEventListener('DOMContentLoaded', () => {
    new FeedReader();
});
