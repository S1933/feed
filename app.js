class FeedReader {
    constructor() {
        this.feeds = [];
        this.currentFeed = null;
        this.cache = null;
        this.init();
    }

    async init() {
        await this.loadFeeds();
        await this.loadCache();
        this.renderFeedList();
        this.bindEvents();

        // Auto-select first feed
        if (this.feeds.length > 0) {
            this.selectFeed(0);
        }
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

    // === UI FUNCTIONS ===

    renderFeedList() {
        const container = document.getElementById('feed-list');
        container.innerHTML = this.feeds.map((feed, index) => `
            <button class="feed-btn ${index === this.currentFeed ? 'active' : ''}"
                    data-index="${index}">
                ${feed.name}
            </button>
        `).join('');
    }

    selectFeed(index) {
        this.currentFeed = index;
        this.renderFeedList();
        this.renderArticles();
    }

    renderArticles() {
        const container = document.getElementById('articles');

        if (this.currentFeed === null || !this.feeds[this.currentFeed]) {
            container.innerHTML = '<p class="empty">Sélectionnez un flux</p>';
            return;
        }

        const feed = this.feeds[this.currentFeed];
        const feedData = this.cache?.feeds?.[feed.url];

        if (!feedData) {
            container.innerHTML = `
                <div class="loading">
                    <div class="spinner"></div>
                    <p>Chargement du cache...</p>
                </div>
            `;
            return;
        }

        if (feedData.status === 'error') {
            container.innerHTML = `
                <div class="error">
                    ❌ Erreur: ${this.escapeHtml(feedData.error || 'Flux indisponible')}
                </div>
            `;
            return;
        }

        const articles = feedData.articles || [];
        const lastUpdate = feedData.last_update
            ? this.formatDate(feedData.last_update)
            : 'inconnu';

        container.innerHTML = `
            <div class="status-bar">
                <span>${articles.length} articles</span>
                <span class="cache-badge">🕐 Mis à jour ${lastUpdate}</span>
            </div>
            ${articles.map(a => this.createArticleHTML(a, feed.name)).join('')}
        `;
    }

    createArticleHTML(article, feedName) {
        const title = this.escapeHtml(article.title || 'Sans titre');
        const desc = this.escapeHtml(this.stripHtml(article.description || '').slice(0, 200));
        const date = article.date ? this.formatDate(article.date) : '';

        return `
            <article class="article">
                <a href="${article.link || '#'}" target="_blank" rel="noopener">
                    <span class="site-badge">${this.escapeHtml(feedName)}</span>
                    <h3>${title}</h3>
                    ${date ? `<time>${date}</time>` : ''}
                    <p>${desc}...</p>
                </a>
            </article>
        `;
    }

    // === EVENTS ===

    bindEvents() {
        // Feed selection
        document.getElementById('feed-list').addEventListener('click', (e) => {
            const btn = e.target.closest('.feed-btn');
            if (!btn) return;
            this.selectFeed(parseInt(btn.dataset.index));
        });

        // Refresh cache
        document.getElementById('refresh-btn').addEventListener('click', async () => {
            await this.loadCache();
            this.renderArticles();
        });
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
            const diff = (now - d) / 1000; // seconds

            if (diff < 60) return "à l'instant";
            if (diff < 3600) return `il y a ${Math.floor(diff / 60)} min`;
            if (diff < 86400) return `il y a ${Math.floor(diff / 3600)}h`;

            return d.toLocaleDateString('fr-FR', {
                day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit'
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
