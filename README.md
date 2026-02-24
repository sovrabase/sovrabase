
<div align="center">
  <h1>🏰 Sovrabase</h1>
  
  <p>
    <strong>La couche de Gouvernance et d'API instantanée pour vos données.</strong>
  </p>
  
  <p>
    Transformez n'importe quelle base de données (PostgreSQL, Mongo) en un backend sécurisé, documenté et souverain en quelques secondes.
  </p>

  <p>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/license-AGPLv3-blue.svg" alt="License">
    </a>
    <a href="https://go.dev">
      <img src="https://img.shields.io/badge/Made%20with-Go-00ADD8.svg" alt="Go">
    </a>
    <a href="#">
      <img src="https://img.shields.io/badge/Docker-Ready-2496ED.svg" alt="Docker">
    </a>
    <a href="#">
      <img src="https://img.shields.io/badge/Status-Alpha-orange.svg" alt="Status">
    </a>
  </p>
</div>

---

## 🧐 Qu'est-ce que Sovrabase ?

Sovrabase n'est pas "juste une autre base de données". C'est un **Data Gateway Souverain**.

Contrairement à Firebase ou Supabase qui vous obligent à migrer vos données chez eux, **Sovrabase se connecte à votre infrastructure existante**. Vous gardez vos données chez OVH, Scaleway, AWS ou sur votre serveur local, et Sovrabase ajoute instantanément :

1.  **Une API REST** auto-générée.
2.  **Une couche d'Authentification & RBAC** (Rôles et Permissions) granulaire.
3.  **Une documentation API** (Swagger/OpenAPI) en temps réel.

C'est la solution idéale pour les développeurs soucieux du **RGPD**, de la **souveraineté numérique** et de la **performance**.

## Architecture

Sovrabase agit comme un middleware intelligent entre vos utilisateurs et votre base de données.



## ✨ Fonctionnalités Clés

- **🔌 Bring Your Own Database (BYODB)** : Connectez-vous à PostgreSQL, MongoDB ou MySQL existants sans migration.
- **🛡️ Gouvernance Granulaire** : Définissez qui peut voir/modifier quoi (Row Level Security) via une interface simple.
- **⚡ Performance Native** : Écrit en Go, compilé en binaire statique. Faible empreinte mémoire, latence minimale.
- **🌍 Souveraineté Totale** : Hébergez-le sur un simple VPS (Hetzner, Scaleway) ou chez vous. Aucune dépendance externe, aucun appel vers les US.
- **📄 Auto-Documentation** : Votre schéma de base de données change ? Votre documentation API se met à jour instantanément.
- **real-time** : Support WebSocket natif pour les mises à jour en temps réel (à venir).

## 🚀 Quick Start (5 minutes)

Prérequis : Docker & Docker Compose.

### 1. Lancer la stack (Risque de changer)

```bash
# Cloner le dépôt
git clone [https://github.com/ketsuna-org/sovrabase.git](https://github.com/ketsuna-org/sovrabase.git)
cd sovrabase

# Démarrer le serveur
docker compose up -d

```

### 2. Connecter votre base de données

Par défaut, l'interface d'administration est accessible sur `http://localhost:8080/_admin`.

1. Connectez-vous (Entrez les identifiants que vous souhaitez).
2. Allez dans "Data Sources" -> "Add New".
3. Entrez votre string de connexion (ex: `postgres://user:pass@host:5432/db`).
4. 🚀 Votre API est prête sur `http://localhost:8080/api/v1/votre-table`.

### 3. Exemple de requête

```bash
curl -X GET http://localhost:8080/api/project/v1/products \
  -H "Authorization: Bearer <votre_token>"

```

## 🆚 Pourquoi Sovrabase ?

| Fonctionnalité | Firebase / Supabase | Strapi / CMS Headless | **Sovrabase** |
| --- | --- | --- | --- |
| **Philosophie** | "Hébergez tout chez nous" | "Gérez du contenu" | **"Gardez vos données, on gère l'accès"** |
| **Données** | Lock-in (propriétaire ou migré) | Stockage interne souvent requis | **Agnostique (Postgres, Mongo, etc.)** |
| **Hébergement** | Cloud (US/Global) | Self-hostable (NodeJS lourd) | **Self-hostable (Go binaire léger)** |
| **Performance** | Variable (Cold starts) | Moyenne | **Haute (Compilé, Connection Pooling)** |

## 🔐 Configuration V1 (Core Foundations)

### Clé maitre obligatoire

Le chiffrement des DSN est actif par defaut. Vous devez fournir une cle 32 bytes via variable d'environnement:

```bash
export SOVRABASE_MASTER_KEY="0123456789abcdef0123456789abcdef"
```

Formats acceptes:
- 32 bytes brut (chaine de 32 caracteres)
- 64 caracteres hex
- base64 d'une cle de 32 bytes

### Secret JWT obligatoire

L'authentification admin utilise un secret dedie:

```bash
export SOVRABASE_JWT_SECRET="change-me-in-production"
```

La variable est obligatoire au demarrage.

### Flux bootstrap admin (`/config`)

Au premier demarrage, si aucun admin n'existe:
- `GET /config` retourne `bootstrap_required=true`
- `POST /config` cree le premier admin et renvoie un JWT

Une fois initialise:
- `GET /config` retourne `bootstrap_required=false`
- utilisez `POST /auth/login` pour vous connecter

Exemple bootstrap:

```bash
curl -X POST http://localhost:8080/config \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"very-strong-password"}'
```

### CLI local admin (sans token)

Le binaire `sovrabase` expose aussi un mode CLI local qui appelle directement les services internes.

Verifier l'etat bootstrap:

```bash
sovrabase config status
```

Bootstrap initial (cree le premier admin et enregistre l'acteur CLI):

```bash
sovrabase admin bootstrap --name "admin@example.com" --password "very-strong-password"
```

Exemples de commandes admin:

```bash
sovrabase admin create-user --name "user@example.com" --password "another-strong-password" --role "admin"
sovrabase admin list-users
sovrabase admin create-role --name "manager" --description "Manager role"
sovrabase admin create-scope --key "users.read" --description "Read users"
```

Connexion locale (enregistre l'acteur CLI, sans gestion de token manuel):

```bash
sovrabase auth login --name "admin@example.com" --password "very-strong-password"
```

Si aucun admin n'est initialise, les commandes admin retournent:

```text
You should have an admin before performing such commands
```

Le mode serveur HTTP reste disponible via:

```bash
sovrabase serve
```

### Choix du metadata store (SQLite ou PostgreSQL)

Par defaut, Sovrabase utilise SQLite (`metadata_store.driver=sqlite`).
Vous pouvez basculer en PostgreSQL:

```yaml
metadata_store:
  driver: "postgres"
  postgres:
    dsn: "postgres://user:password@host:5432/sovrabase?sslmode=disable"
```

Overrides d'environnement disponibles:
- `SOVRABASE_METADATA_DRIVER`
- `SOVRABASE_METADATA_SQLITE_PATH`
- `SOVRABASE_METADATA_POSTGRES_DSN`

### Mode managed DB via Docker

Deux modes reseau sont supportes:
- `host_port`: publication automatique de ports sur l'hote
- `network`: reseau Docker dedie, DSN resolu via nom de conteneur

Config:

```yaml
provisioning:
  docker:
    mode: "host_port" # ou "network"
    host_address: "127.0.0.1"
    network_name: "sovrabase-managed"
```

Overrides d'environnement:
- `SOVRABASE_DOCKER_MODE`
- `SOVRABASE_DOCKER_HOST_ADDRESS`
- `SOVRABASE_DOCKER_NETWORK_NAME`

## 🛠️ Stack Technique

Construit pour la robustesse et la maintenabilité par une équipe réduite.

* **Langage** : Go 1.25+ (Fiber/Hono inspired router)
* **Containerization** : Docker, Distroless images (sécurité max)
* **Stockage Interne** : SQLite (pour les configs) ou Redis
* **Front Admin** : SvelteKit / Tailwind (Léger et rapide)

## 🗺️ Roadmap

Nous construisons une alternative européenne crédible.

* [ ] Connexion PostgreSQL & MongoDB
* [ ] Génération API REST basique
* [ ] Interface de gestion des rôles (RBAC) UI
* [ ] Support S3 (MinIO, Scaleway Object Storage)
* [ ] SDK Client (JS/TS)
* [ ] Real-time Subscriptions

Consultez le [ROADMAP.md](https://www.google.com/search?q=docs/ROADMAP.md) pour les détails.

## 🤝 Contribuer

Sovrabase est un projet communautaire. Nous cherchons des contributeurs !
Si vous voulez aider à construire l'infrastructure souveraine de demain :

1. Lisez [CONTRIBUTING.md](https://www.google.com/search?q=CONTRIBUTING.md).
2. Prenez une issue taguée `good first issue`.
3. Rejoignez la discussion (Lien Discord/Matrix à venir).

## 📄 Licence

Ce projet est sous licence **AGPLv3**.
Cela signifie que vous pouvez l'utiliser librement, mais si vous modifiez le code source de Sovrabase et que vous offrez ce service à des utilisateurs via un réseau, vous devez partager vos modifications.

---

<div align="center">
Made with ❤️ in 🇫🇷 & 🇪🇺 for developers who value freedom.
</div>
