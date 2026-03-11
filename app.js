class FeedReader {
    constructor() {
        this.feeds = [];
        this.cache = null;
        this.allArticles = [];
        this.readArticles = this.loadReadArticles();
        this.favoriteArticles = this.loadFavoriteArticles();
        this.init();
    }

    // === LOCAL STORAGE ===

    loadReadArticles() {
        try {
            const data = localStorage.getItem('readArticles');
            return data ? JSON.parse(data) : {};
        } catch (e) {
            return {};
        }
    }

    saveReadArticles() {
        localStorage.setItem('readArticles', JSON.stringify(this.readArticles));
    }

    loadFavoriteArticles() {
        try {
            const data = localStorage.getItem('favoriteArticles');
            return data ? JSON.parse(data) : {};
        } catch (e) {
            return {};
        }
    }

    saveFavoriteArticles() {
        localStorage.setItem('favoriteArticles', JSON.stringify(this.favoriteArticles));
    }

    markAsRead(articleId) {
        this.readArticles[articleId] = Date.now();
        this.saveReadArticles();
    }

    isRead(articleId) {
        return !!this.readArticles[articleId];
    }

    toggleFavorite(articleId) {
        if (this.favoriteArticles[articleId]) {
            delete this.favoriteArticles[articleId];
        } else {
            this.favoriteArticles[articleId] = Date.now();
        }
        this.saveFavoriteArticles();
        return this.isFavorite(articleId);
    }

    isFavorite(articleId) {
        return !!this.favoriteArticles[articleId];
    }

    // === INIT ===

    async init() {
        await this.loadFeeds();
        await this.loadCache();
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

        if (articles.length === 0) {
            container.innerHTML = filter === 'favorites'
                ? '<p class="empty">Aucun favori. Cliquez sur ⭐ pour en ajouter !</p>'
                : '<p class="empty">Aucun article</p>';
            return;
        }

        const lastUpdate = this.cache?.last_fetch
            ? this.formatDate(this.cache.last_fetch)
            : 'inconnu';

        const unreadCount = articles.filter(a => !this.isRead(this.getArticleId(a))).length;

        container.innerHTML = `
            <div class="status-bar">
                <span>${unreadCount}/${articles.length} non lus</span>
                <span class="cache-badge">🕐 Mis à jour ${lastUpdate}</span>
            </div>
            ${articles.map(a => this.createArticleHTML(a)).join('')}
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
        document.getElementById('articles').addEventListener('click', (e) => {
            const link = e.target.closest('a');
            if (link) {
                const articleId = link.dataset.id;
                if (articleId) {
                    this.markAsRead(articleId);
                    const article = link.closest('.article');
                    if (article) {
                        article.classList.add('read');
                        this.updateUnreadCount();
                    }
                }
            }

            // Toggle favorite
            const favBtn = e.target.closest('.fav-btn');
            if (favBtn) {
                e.preventDefault();
                e.stopPropagation();
                const articleId = favBtn.dataset.id;
                const isNowFav = this.toggleFavorite(articleId);
                favBtn.innerHTML = isNowFav ? '⭐' : '☆';
                favBtn.classList.toggle('active', isNowFav);
                this.renderFeedList(); // Update fav count
            }
        });
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
