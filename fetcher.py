#!/usr/bin/env python3
"""
Fetch RSS feeds and store them in cache.
Run this with a cron job every hour:
0 * * * * cd /path/to/app && python3 fetcher.py
"""

import json
import time
import xml.etree.ElementTree as ET
from urllib.request import urlopen, Request
from urllib.error import URLError
from datetime import datetime

# Liste des flux RSS
FEEDS = [
    {"name": "Sud Ouest Bordeaux", "url": "http://www.sudouest.fr/gironde/bordeaux/rss.xml"},
    {"name": "Courrier International", "url": "http://www.courrierinternational.com/rss/all/rss.xml"},
    {"name": "YoanDev", "url": "https://flux.yoandev.co/rss.xml"},
    {"name": "France Info Sciences", "url": "http://www.francetvinfo.fr/sciences.rss"},
    {"name": "Hacker News", "url": "https://hnrss.org/frontpage"},
    {"name": "Le Monde IA", "url": "http://www.lemonde.fr/intelligence-artificielle/rss_full.xml"},
    {"name": "Korben", "url": "http://feeds.feedburner.com/KorbensBlog-UpgradeYourMind"},
    {"name": "Rue89 Bordeaux", "url": "http://feeds.feedburner.com/Rue89Bordeaux"},
    {"name": "Le Monde Sciences", "url": "http://www.lemonde.fr/rss/tag/sciences.xml"},
    {"name": "AWS Blog", "url": "http://blogs.aws.amazon.com/application-management/blog/feed/recentPosts.rss"},
    {"name": "Le Figaro Sciences", "url": "http://www.lefigaro.fr/rss/figaro_sciences.xml"},
]

CACHE_FILE = "cache.json"


def fetch_feed(feed_url):
    """Fetch and parse a single RSS feed."""
    try:
        headers = {
            'User-Agent': 'Mozilla/5.0 (compatible; RSS Reader)'
        }
        req = Request(feed_url, headers=headers)

        with urlopen(req, timeout=30) as response:
            xml_content = response.read()

        # Parse XML
        root = ET.fromstring(xml_content)

        # Detect Atom vs RSS
        is_atom = root.tag.endswith('feed')

        articles = []

        if is_atom:
            entries = root.findall('.//{http://www.w3.org/2005/Atom}entry')
            for entry in entries[:20]:  # Max 20 articles
                title = entry.find('{http://www.w3.org/2005/Atom}title')
                link = entry.find('{http://www.w3.org/2005/Atom}link')
                summary = entry.find('{http://www.w3.org/2005/Atom}summary') or entry.find('{http://www.w3.org/2005/Atom}content')
                updated = entry.find('{http://www.w3.org/2005/Atom}updated') or entry.find('{http://www.w3.org/2005/Atom}published')

                articles.append({
                    "title": title.text if title is not None else "Sans titre",
                    "link": link.get('href') if link is not None else "#",
                    "description": summary.text if summary is not None else "",
                    "date": updated.text if updated is not None else ""
                })
        else:
            items = root.findall('.//item')
            for item in items[:20]:  # Max 20 articles
                title = item.find('title')
                link = item.find('link')
                description = item.find('description')
                pubDate = item.find('pubDate')

                articles.append({
                    "title": title.text if title is not None else "Sans titre",
                    "link": link.text if link is not None else "#",
                    "description": description.text if description is not None else "",
                    "date": pubDate.text if pubDate is not None else ""
                })

        return {
            "articles": articles,
            "last_update": datetime.now().isoformat(),
            "status": "ok",
            "error": None
        }

    except Exception as e:
        return {
            "articles": [],
            "last_update": datetime.now().isoformat(),
            "status": "error",
            "error": str(e)
        }


def update_cache():
    """Update cache for all feeds."""
    cache = {
        "last_fetch": datetime.now().isoformat(),
        "feeds": {}
    }

    for feed in FEEDS:
        print(f"Fetching: {feed['name']}...", end=" ")
        start_time = time.time()

        result = fetch_feed(feed["url"])
        cache["feeds"][feed["url"]] = result

        elapsed = time.time() - start_time
        if result["status"] == "ok":
            print(f"✓ {len(result['articles'])} articles ({elapsed:.1f}s)")
        else:
            print(f"✗ Error: {result['error'][:50]}")

    # Save cache
    with open(CACHE_FILE, "w", encoding="utf-8") as f:
        json.dump(cache, f, ensure_ascii=False, indent=2)

    print(f"\nCache saved to {CACHE_FILE}")
    print(f"Next update: {(datetime.now().hour + 1) % 24}:00")


if __name__ == "__main__":
    print(f"=== RSS Fetcher - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} ===\n")
    update_cache()
