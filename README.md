# 📰 RSS Reader

Un lecteur RSS simple et rapide avec backend Go, cache automatique, favoris et interface responsive.

## ✨ Fonctionnalités

- 🚀 **Backend Go** : Ultra rapide, léger (~10MB RAM)
- 🔄 **Cache automatique** : Mise à jour des flux toutes les heures
- ⭐ **Favoris** : Sauvegardez vos articles préférés
- 📱 **Responsive** : Sidebar verticale sur PC, scroll horizontal sur mobile
- 👁️ **Articles lus** : Marquage manuel, articles grisés
- 🏷️ **Filtrage** : Par flux ou "Tous" mélangés
- 🌙 **Dark mode** : Support natif du mode sombre

## 🚀 Installation

### Prérequis
- Go 1.21+ : https://go.dev/dl/

### Lancer l'application

```bash
# Cloner le repo
git clone [url-du-repo]
cd feed

# Lancer le serveur Go
go run main.go
```

Accédez à l'application : **http://localhost:7007**

### Compiler (optionnel)

```bash
# Créer un exécutable
go build -o rss-server
./rss-server
```

## 📁 Structure

```
.
├── main.go         # Serveur Go + scheduler + fetcher RSS
├── go.mod          # Module Go
├── app.js          # Application frontend
├── index.html      # Page principale
├── styles.css      # Styles responsive
├── cache.json      # Cache des articles (auto-généré)
├── .gitignore      # Fichiers ignorés par git
└── README.md       # Ce fichier
```

## ⚙️ Configuration

### Modifier les flux RSS

Éditez [`main.go`](main.go:46) :

```go
var feeds = []Feed{
    {Name: "Mon Flux", URL: "https://example.com/rss.xml"},
    // ...
}
```

### Changer le port

Éditez [`main.go`](main.go:91) :

```go
port := "7007"  // Port par défaut
```

## 🖥️ Utilisation

### Sur PC
- Sidebar verticale à gauche avec tous les flux
- Cliquez sur un flux pour filtrer
- Cliquez sur "⭐ Favoris" pour voir vos favoris
- Cliquez sur un article pour le marquer comme lu (devient gris)
- Cliquez sur ⭐ pour ajouter/retirer des favoris

### Sur mobile (iPhone/Android)
- Barre de flux horizontale scrollable en haut
- Swipez pour changer de flux
- Même fonctionnalités que sur PC

### Raccourcis
| Action | Résultat |
|--------|----------|
| Clic sur article | Ouvre l'article + marque comme lu |
| Clic sur ⭐ | Ajoute/retire des favoris |
| Clic sur flux | Filtre les articles |

## 🔧 Fonctionnement technique

### Backend Go

Le serveur Go gère :
- **HTTP Server** : Servir les fichiers statiques et l'API
- **API REST** : `/api/feeds` et `/api/cache`
- **RSS Fetcher** : Récupère et parse les flux RSS/Atom
- **Cache** : Stockage JSON avec mise à jour automatique
- **Goroutines** : Background updater toutes les heures

```go
main.go
├── HTTP Server (port 7007)
│   ├── Static files (/, /app.js, /styles.css)
│   ├── /api/feeds → Liste des flux
│   └── /api/cache → Articles en cache
├── RSS Parser
│   ├── parseRSS()   → XML RSS 2.0
│   └── parseAtom()  → XML Atom
└── Cache System
    ├── updateCache()        → Fetch tous les flux
    ├── backgroundUpdater()  → Goroutine hourly
    └── saveCache/loadCache  → JSON file
```

### Cache
- Les flux sont récupérés toutes les heures automatiquement
- Stockage dans `cache.json`
- Mise à jour au démarrage du serveur

### Stockage local (navigateur)
- **Articles lus** : `localStorage.readArticles`
- **Favoris** : `localStorage.favoriteArticles`
- Persistance après fermeture du navigateur

## 📡 API

Le serveur expose 2 endpoints :

### `GET /api/feeds`
Retourne la liste des flux configurés.

```json
{
  "feeds": [
    {"name": "Sud Ouest", "url": "..."},
    ...
  ],
  "count": 11
}
```

### `GET /api/cache`
Retourne les articles en cache.

```json
{
  "last_fetch": "2026-03-11T12:00:00",
  "feeds": {
    "http://...": {
      "articles": [...],
      "last_update": "...",
      "status": "ok"
    }
  }
}
```

## 🐛 Dépannage

### Le serveur ne démarre pas
```bash
# Vérifier si le port est utilisé
lsof -i :7007

# Changer de port dans main.go
port := "7008"
```

### Les flux ne se mettent pas à jour
```bash
# Supprimer le cache manuellement
rm cache.json

# Redémarrer le serveur
go run main.go
```

### Accès depuis un autre appareil
```bash
# Trouver l'IP locale
ip addr show | grep "inet " | head -1

# Accéder depuis iPhone/tablet
# http://[IP_LOCALE]:7007
```

## 📝 License

MIT
