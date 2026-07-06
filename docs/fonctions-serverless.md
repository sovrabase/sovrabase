# Fonctions Serverless (JavaScript & Go)

Sovrabase permet d'exécuter du **JavaScript pur** ou du **Go interprété** côté serveur, avec une expérience proche de Firebase Cloud Functions : déploiement par projet, déclencheurs HTTP et événements, métriques et logs d'exécution.

> **Moteurs** : [goja](https://github.com/dop251/goja) (JavaScript ES5.1+) et [yaegi](https://github.com/traefik/yaegi) (Go interprété), intégrés au binaire Go sans Node.js.

---

## Table des matières

1. [Vue d'ensemble](#vue-densemble)
2. [Créer une fonction](#créer-une-fonction)
3. [Déployer](#déployer)
4. [Déclencheurs (triggers)](#déclencheurs-triggers)
5. [Contraintes d'exécution](#contraintes-dexécution)
6. [Exemples](#exemples)
7. [Développement local](#développement-local)
8. [API de référence](#api-de-référence)
9. [Limitations et feuille de route](#limitations-et-feuille-de-route)

---

## Vue d'ensemble

| Concept | Description |
|---------|-------------|
| **Fonction** | Script JS ou Go stocké par projet dans PebbleDB |
| **Langage** | `javascript` (défaut) ou `go` |
| **Trigger** | Événement qui déclenche l'exécution |
| **Exécuteur** | Runtime sandboxé (goja / yaegi) avec timeout |
| **Métriques** | RAM, CPU, durée par exécution + agrégats horaires |
| **Logs** | Historique des 200 dernières exécutions par projet |

**URL d'invocation HTTP** (fonctions `http`) :

```
GET|POST|PUT|DELETE  /api/v1/functions/{nom}
Header: X-Project-Key: <clé-projet>
Header: Authorization: Bearer <jwt>   (si public=false)
```

---

## Créer une fonction

### Fonction Go (HTTP)

```go
package function

func Handler(req map[string]interface{}) map[string]interface{} {
    return map[string]interface{}{
        "status": 200,
        "body":   map[string]interface{}{"ok": true},
    }
}
```

> Limites yaegi : pas de `unsafe`, pas de goroutines libres, subset du langage Go standard.

### Fonction HTTP

Le fichier JS doit définir une fonction globale `handler` :

```javascript
function handler(req) {
  // req.method, req.path, req.query, req.headers, req.body
  return {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    body: { ok: true, echo: req.body },
  };
}
```

### Fonction événementielle

Le code s'exécute avec une variable globale `event` :

```javascript
// trigger: record:create, collection: posts
console.log('Nouveau post:', event.id, event.data.title);
```

Structure de `event` :

```json
{
  "type": "record:create",
  "collection": "posts",
  "id": "abc123",
  "data": { "title": "Hello" },
  "user_id": "user-uuid",
  "user_email": "user@example.com"
}
```

---

## Déployer

### Via l'API Admin (recommandé)

```bash
curl -X POST "http://localhost:6070/admin/projects/{PROJECT_ID}/functions" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "hello",
    "trigger": "http",
    "public": true,
    "timeout_ms": 5000,
    "code": "function handler(req) { return { status: 200, body: { msg: \"hi\" } }; }"
  }'
```

### Script d'exemple

Voir [`examples/functions/deploy.ps1`](../examples/functions/deploy.ps1) et les fichiers JS associés.

### Mettre à jour / supprimer

```bash
# Mettre à jour
curl -X PUT ".../functions/{FUNCTION_ID}" -d '{"code": "...", "enabled": true}'

# Supprimer
curl -X DELETE ".../functions/{FUNCTION_ID}"
```

---

## Déclencheurs (triggers)

| Trigger | Quand | Paramètres |
|---------|-------|------------|
| `http` | Appel HTTP direct | `public` (bool) |
| `record:create` | Insertion document | `collection` (optionnel, `*` = toutes) |
| `record:update` | Mise à jour document | `collection` |
| `record:delete` | Suppression document | `collection` |
| `auth:signup` | Inscription utilisateur | — |
| `auth:signin` | Connexion utilisateur | — |
| `schedule` | Planifié (cron) | `schedule` (expression 5 champs) — *voir ci-dessous* |

### Planification (cron)

Le trigger **`schedule`** est exécuté nativement par le `ScheduleWatcher` (tick 30 s, expression cron 5 champs).

Alternative : **job cron HTTP** (onglet Cron) pointant vers une fonction HTTP :

```json
{
  "name": "daily-cleanup",
  "schedule": "0 3 * * *",
  "url": "http://localhost:6070/api/v1/functions/cleanup?project_key=VOTRE_CLE",
  "method": "POST"
}
```

---

## Bindings sandbox

| API | JS | Go |
|-----|:--:|:--:|
| `sovrabase.db.get/create/update/delete` | ✅ | via host |
| `sovrabase.storage.upload/download/delete/list` | ✅ | ❌ (MVP JS) |
| `fetch(url, opts)` | ✅ | via symbole host |

### Storage (JS)

```javascript
function handler(req) {
  // upload(bucket, path, data, { contentType? })
  sovrabase.storage.upload("images", "pic.png", binaryData, { contentType: "image/png" });

  // download → { data, bucket, path, size, content_type, url, ... }
  const file = sovrabase.storage.download("images", "pic.png");

  sovrabase.storage.delete("images", "pic.png");
  const files = sovrabase.storage.list("images", "prefix/");
  return { status: 200, body: { count: files.length } };
}
```

Limite sandbox : **10 Mo** par upload/download via RPC (REST API upload = 50 Mo).

---

## Timeout coopératif

Les fonctions JS utilisent `vm.Interrupt` dès expiration du contexte d'exécution. Les appels host (`db`, `fetch`, `storage`) sont annulables via RPC (fermeture connexion worker).

| Runtime | Comportement au timeout |
|---------|-------------------------|
| **JavaScript** | Interrupt goja + annulation RPC ; kill subprocess en filet (+500 ms) |
| **Go (yaegi)** | Kill subprocess uniquement (pas d'interrupt coopératif) |

---

## Métriques d'exécution

Chaque exécution est lancée dans un **sous-processus worker** (`sovrabase __fn_worker__`). Les métriques mesurent le **RSS et le CPU réels de ce worker**, pas le processus Sovrabase principal.

| Champ | Description |
|-------|-------------|
| `duration_ms` | Durée wall-clock |
| `mem_peak_bytes` | Pic RSS du worker (mémoire résidente OS) |
| `cpu_user_ms` / `cpu_sys_ms` | CPU cumulé du worker |
| `samples[]` | Points toutes les 100 ms (max 60), RSS + CPU |
| `isolated` | `true` quand mesuré depuis le subprocess worker |

Les bindings (`sovrabase.db`, `sovrabase.storage`, `fetch`) passent par un canal RPC TCP local vers le processus parent.

Mode fallback (tests) : `SetIsolatedExecution(false)` — exécution in-process avec métriques du processus courant (`isolated: false`).

Variable d'environnement optionnelle : `SOVRABASE_FN_WORKER_EXE` — chemin du binaire worker (utile pour les tests d'intégration).

Dashboard : onglet **Functions** → graphiques 24h/7j + drill-down par exécution.

API : `GET .../functions/{fid}/metrics?range=24h|7d` et `GET .../executions/{execId}`.

---

## Contraintes d'exécution

| Paramètre | Valeur |
|-----------|--------|
| **Timeout par défaut** | 5 000 ms |
| **Timeout maximum** | 30 000 ms |
| **Taille du code** | Limitée par le body HTTP (10 Mo max serveur) |
| **Concurrence** | Événements asynchrones (goroutines) |
| **Réseau** | ✅ `fetch` (HTTP sortant contrôlé) |
| **Modules npm** | ❌ Non supportés |
| **APIs disponibles** | `console.log`, `JSON`, `sovrabase.db`, `fetch` |
| **Langages** | JavaScript (goja), Go (yaegi) |
| **Auth HTTP** | `public: true` = sans JWT ; sinon Bearer requis |

Les logs `console.log` apparaissent dans les logs serveur Sovrabase (`function log`).

---

## Exemples

### Hello World (HTTP public)

```bash
curl "http://localhost:6070/api/v1/functions/hello?project_key=CLE&name=Europe"
```

Réponse :

```json
{ "message": "Bonjour Europe !", "method": "GET" }
```

### Webhook de validation (HTTP privé)

```javascript
function handler(req) {
  const secret = req.headers['x-webhook-secret'];
  if (secret !== 'mon-secret') {
    return { status: 403, body: { error: 'forbidden' } };
  }
  return { status: 200, body: { received: req.body } };
}
```

Déployez avec `"public": false` et appelez avec un JWT utilisateur valide.

### Notification à la création d'utilisateur

```javascript
// trigger: record:create, collection: users
console.log('Utilisateur créé:', event.data.email);
```

### Tester depuis l'admin

```bash
curl -X POST ".../functions/{FUNCTION_ID}/test" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"method":"POST","body":{"test":true}}'
```

---

## Développement local

1. **Démarrer Sovrabase** : `make run` ou `go run ./cmd/sovrabase`
2. **Créer un projet** via le dashboard (`http://localhost:6070`)
3. **Déployer** avec le script `examples/functions/deploy.ps1`
4. **Tester** :
   - HTTP : `curl` vers `/api/v1/functions/{nom}`
   - Événement : insérer un document via SDK ou dashboard
   - Admin test : `POST .../functions/{id}/test`
5. **Consulter les exécutions** : `GET .../functions/{id}/executions`

### Déboguer

- Vérifiez les logs stdout du serveur pour `function log`
- Consultez `executions` pour status/durée/erreur
- Réduisez le code à un `console.log(event)` minimal pour isoler les problèmes

---

## API de référence

### Admin

| Méthode | Endpoint | Description |
|---------|----------|-------------|
| `GET` | `/admin/projects/{id}/functions` | Lister |
| `POST` | `/admin/projects/{id}/functions` | Créer |
| `GET` | `/admin/projects/{id}/functions/{fid}` | Détail |
| `PUT` | `/admin/projects/{id}/functions/{fid}` | Modifier |
| `DELETE` | `/admin/projects/{id}/functions/{fid}` | Supprimer |
| `POST` | `/admin/projects/{id}/functions/{fid}/test` | Test manuel |
| `GET` | `/admin/projects/{id}/functions/{fid}/executions` | Historique |
| `GET` | `/admin/projects/{id}/functions/{fid}/executions/{eid}` | Détail + samples |
| `GET` | `/admin/projects/{id}/functions/{fid}/metrics?range=24h` | Agrégats horaires |
| `GET` | `/admin/projects/{id}/functions/metrics/summary` | Résumé projet |

### Public (HTTP trigger)

| Méthode | Endpoint | Auth |
|---------|----------|------|
| `*` | `/api/v1/functions/{name}` | `X-Project-Key` + JWT si `public=false` |

### Modèle Function

```typescript
interface Function {
  id: string;
  name: string;           // [a-z][a-z0-9_-]{1,62}
  language?: string;      // javascript | go
  trigger: string;
  collection?: string;
  schedule?: string;      // cron 5 champs (trigger schedule)
  code: string;
  public: boolean;
  timeout_ms: number;     // défaut 5000, max 30000
  enabled: boolean;
  created_at: string;
  updated_at: string;
}
```

---

## Limitations et feuille de route

**Actuellement non supporté :**

- `require()` / modules npm
- WASM utilisateur (prévu)
- Isolation cgroup / limites mémoire strictes (worker subprocess sans cgroup)
- Hot reload automatique

**Prévu :**

- CLI `sovrabase functions deploy`
- Runtime WASM (wazero)

---

## Comparaison Firebase Functions

| Firebase | Sovrabase |
|----------|-----------|
| `exports.hello = functions.https.onRequest(...)` | `handler(req)` + trigger `http` |
| `onCreate` Firestore | `record:create` + `collection` |
| `onAuth` | `auth:signup` / `auth:signin` |
| `functions.pubsub.schedule` | Trigger `schedule` natif |
| `firebase deploy --only functions` | Dashboard Functions ou API Admin |
| Node.js 18+ | goja ES5.1+ / yaegi Go |
