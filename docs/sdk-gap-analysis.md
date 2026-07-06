# Analyse des écarts API ↔ SDKs

État au 6 juillet 2026. Les **3 SDKs** officiels sont :

| SDK | Emplacement | Langage |
|-----|-------------|---------|
| **Go** | `sovrabase/client/` | Go (référence, dans le monorepo) |
| **JavaScript/TypeScript** | `sovrabase-js-sdk/` | npm `@sovrabase/js-sdk` |
| **Dart/Flutter** | `sovrabase-dart-sdk/` | pub `sovrabase_client` |

Il n'existe pas de 4ᵉ SDK client dans le dépôt. Le README mentionne la génération OpenAPI pour d'autres langages.

---

## Surface API publique (`/api/v1`, `/auth/v1`, `/realtime/v1`)

### Auth (`/auth/v1`)

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| signup, signin, refresh | ✅ | ✅ | ✅ |
| MFA (setup, confirm, disable, status, challenge) | ✅ | ✅ | ✅ |
| Magic link | ✅ | ✅ | ✅ |
| OAuth URL | ✅ | ✅ | ✅ |
| forgot/reset password, verify email | ✅ | ✅ | ✅ |
| getMe / updateMe | ✅ | ✅ | ✅ |
| signIn → SignInResult (MFA) | ✅ | ✅ | ✅ |

### Base de données

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| CRUD + PATCH | ✅ | ✅ | ✅ |
| list, query, search, batch | ✅ | ✅ | ✅ |

### Storage

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| upload, download, list, delete | ✅ | ✅ | ✅ |
| signed-url | ✅ | ✅ | ✅ |

### Realtime WebSocket

| Fonctionnalité | Go | JS | Dart |
|----------------|:--:|:--:|:----:|
| connect, subscribe, unsubscribe | ✅ | ✅ | ✅ |
| reconnexion auto | ❌ | ✅ | ✅ |
| filtres subscription | ❌ | ❌ | ❌ |

### Queues

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| send, receive, delete | ✅ | ✅ | ✅ |

### Remote Config

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| list, get, set, delete, public | ✅ | ✅ | ✅ |

### Analytics

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| ingest events | ✅ | ✅ | ✅ |

### Integrations

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| get integrations | ✅ | ✅ | ✅ |
| integration action | ✅ | ✅ | ✅ |

### Fonctions serverless

| Endpoint | Go | JS | Dart |
|----------|:--:|:--:|:----:|
| invoke HTTP function | ✅ | ✅ | ✅ |
| admin CRUD functions | ❌ | ❌ | ❌ |

> Admin CRUD et métriques : dashboard (onglets Functions / Cron) ou API Admin directe.

---

## API Admin (`/admin/*`)

Non couverte par les SDKs clients (par design — réservée au dashboard et aux outils ops).

---

## Incohérences corrigées

| Problème | Correction |
|----------|------------|
| Go SDK : `/auth/v1/mfa/complete` (inexistant) | → `/auth/v1/mfa/challenge` |
| Go SDK : `/auth/v1/signin-mfa` (inexistant) | → `/auth/v1/signin` |
| JS/Dart : `signIn` retournait `TokenPair` sans MFA | → `SignInResult` |
| JS/Dart : méthodes manquantes (queues, config, etc.) | Ajoutées |

---

## Travail restant (priorité)

1. **Realtime Go** : reconnexion automatique (parité JS/Dart)
2. **Types** : harmoniser `User` (champs `name`, `avatar_url` partout)
3. **Tests** : couverture MFA, queues, config, fonctions dans JS/Dart
4. **OpenAPI** : régénérer `swagger.json` avec endpoints functions
5. **Runtime WASM** : fonctions Go compilées (wazero)
