#!/usr/bin/env python3
"""
Serveur RSS avec cache intégré.
Met à jour les flux toutes les heures automatiquement.
"""

import http.server
import socketserver
import json
import threading
import time
from datetime import datetime, timedelta
from fetcher import update_cache, FEEDS, CACHE_FILE

PORT = 7007
UPDATE_INTERVAL = 3600  # 1 hour in seconds


def background_updater():
    """Background thread that updates cache every hour."""
    while True:
        try:
            print(f"[{datetime.now().strftime('%H:%M:%S')}] 🔄 Updating cache...")
            update_cache()
            print(f"[{datetime.now().strftime('%H:%M:%S')}] ✅ Cache updated. Next: {(datetime.now() + timedelta(hours=1)).strftime('%H:%M')}")
        except Exception as e:
            print(f"[{datetime.now().strftime('%H:%M:%S')}] ❌ Update error: {e}")

        # Sleep for 1 hour
        time.sleep(UPDATE_INTERVAL)


def start_background_updater():
    """Start the background updater thread."""
    # Initial cache update
    try:
        print("📦 Initial cache update...")
        update_cache()
    except Exception as e:
        print(f"⚠️ Initial cache failed: {e}")

    # Start background thread
    thread = threading.Thread(target=background_updater, daemon=True)
    thread.start()
    print("📅 Background updater started (every hour)\n")


class RSSHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header('Access-Control-Allow-Origin', '*')
        super().end_headers()

    def log_message(self, format, *args):
        print(f"[{datetime.now().strftime('%H:%M:%S')}] {args[0]}")

    def do_GET(self):
        # API: List of feeds
        if self.path == '/api/feeds':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()
            response = {"feeds": FEEDS, "count": len(FEEDS)}
            self.wfile.write(json.dumps(response).encode())
            return

        # API: Cached articles
        if self.path == '/api/cache':
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.end_headers()

            try:
                with open(CACHE_FILE, 'r', encoding='utf-8') as f:
                    cache_data = json.load(f)
                self.wfile.write(json.dumps(cache_data).encode())
            except FileNotFoundError:
                self.wfile.write(json.dumps({
                    "error": "Cache not ready",
                    "feeds": {},
                    "last_fetch": None
                }).encode())
            return

        # Static files
        super().do_GET()


if __name__ == "__main__":
    print(f"=== RSS Server - {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} ===\n")

    # Start background cache updater
    start_background_updater()

    # Start HTTP server
    with socketserver.TCPServer(("", PORT), RSSHandler) as httpd:
        print(f"🌐 Server running at http://localhost:{PORT}")
        print("Press Ctrl+C to stop\n")

        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\n\n👋 Shutting down...")
