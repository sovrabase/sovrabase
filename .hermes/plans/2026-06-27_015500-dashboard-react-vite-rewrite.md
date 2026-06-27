# Plan: Dashboard React + Vite Rewrite

**Goal**: Remplacer le dashboard vanilla JS (7096 lignes) par React 19 + Vite + Tailwind v4.

## Contexte

| Actuel | Cible |
|--------|-------|
| 1 fichier HTML 1754 lignes | Composants React modulaires |
| 6 fichiers JS vanilla (7096 lignes) | TS éventuellement, sinon JSX |
| 1 fichier CSS 1374 lignes | Tailwind utility-first |
| `//go:embed` binaire unique | `dist/` embed par Go |
| 76 fonctions dans `app.js` | Hooks + Zustand store |
| ~43 appels API | Même wrapper `api()` |

## Architecture

```
sovrabase-dashboard/          → nouveau repo ou /frontend/ dans sovrabase
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── tsconfig.json
├── package.json
├── src/
│   ├── main.tsx              → entry point
│   ├── App.tsx               → router + layout
│   ├── api.ts                → fetch wrapper (remplace api.js)
│   ├── store.ts              → Zustand (auth, projects, activeProject)
│   ├── types.ts              → interfaces (Project, User, Bucket, etc.)
│   ├── pages/
│   │   ├── Login.tsx
│   │   ├── Dashboard.tsx     → stats cards + replication + usage
│   │   ├── Projects.tsx      → CRUD table + modals
│   │   ├── Settings.tsx      → 8 tabs (admin, admins, security, s3, smtp, replication, audit, backups)
│   │   └── Plugins.tsx       → plugins + hooks + routes tables
│   ├── project/
│   │   ├── ProjectDetail.tsx → layout + tab bar
│   │   ├── OverviewTab.tsx
│   │   ├── TeamTab.tsx
│   │   ├── DatabaseTab.tsx
│   │   ├── AuthTab.tsx
│   │   ├── StorageTab.tsx
│   │   ├── ConfigTab.tsx
│   │   ├── CronTab.tsx
│   │   ├── WebhooksTab.tsx
│   │   ├── QueuesTab.tsx
│   │   ├── AnalyticsTab.tsx
│   │   ├── ApiTab.tsx
│   │   └── LogsTab.tsx
│   ├── components/
│   │   ├── Layout.tsx        → sidebar nav + content area
│   │   ├── Modal.tsx
│   │   ├── Toast.tsx
│   │   ├── ConfirmDialog.tsx
│   │   ├── StatCard.tsx
│   │   ├── TabBar.tsx
│   │   ├── DataTable.tsx
│   │   └── ui/               → shadcn/ui components
│   └── hooks/
│       ├── useApi.ts
│       └── useToast.ts
└── dist/                     → output embeddé par Go
```

## Design System (repris de l'existant)

```
--bg-app: #0a0a0f
--bg-card: #141416
--border: #222226
--text-primary: #f0f0f5
--text-secondary: #8b8b96
--text-muted: #5c5c66
--accent: #5b5bff
--success: #22c55e
--danger: #ef4444
--font: 'Inter', -apple-system, sans-serif
--font-mono: 'JetBrains Mono', monospace
--radius: 8px
--radius-xl: 16px
```

Glassmorphism login card, dark gradient background, lucide-react icons.

## Plan d'exécution (8 phases)

### Phase 1: Scaffolding (5 min)
- `npm create vite@latest sovrabase-dashboard -- --template react-ts`
- Install deps: `react-router-dom`, `zustand`, `lucide-react`, `tailwindcss @tailwindcss/vite`
- Config Tailwind dark theme
- Créer arborescence `src/`

### Phase 2: API Layer + Store (5 min)
- `api.ts` : même comportement que `api.js` (token en localStorage, JSON, error handling)
- `store.ts` : Zustand store (auth token, projects list, current project detail)
- `types.ts` : toutes les interfaces

### Phase 3: Layout + Navigation (10 min)
- `Layout.tsx` : sidebar (Dashboard, Projects, Settings, Plugins) + version/region footer
- `App.tsx` : React Router v7 routes
- Auth guard : redirige vers `/login` si pas de token
- `Login.tsx` : même glassmorphism card (repris du HTML existant)

### Phase 4: Pages simples (15 min)
- `Dashboard.tsx` : 3 stat cards + replication info + usage stats
- `Projects.tsx` : table CRUD + modal create + delete confirm + API key reveal
- `Settings.tsx` : TabBar 8 tabs — chaque tab = formulaire PATCH
- `Plugins.tsx` : cartes plugins + table hooks color-coded + table routes

### Phase 5: Project Detail Tabs (30 min) — le plus gros morceau
- `ProjectDetail.tsx` : TabBar 12 tabs + data fetching
- Chaque `*Tab.tsx` : fetch → render table/form — structure identique au JS existant
- Ordre: Overview, Team, Database, Auth, Storage, Config, Cron, Webhooks, Queues, Analytics, API, Logs

### Phase 6: Composants partagés (10 min)
- `Modal.tsx` : overlay + close animation (comme `closing` class existante)
- `Toast.tsx` : notifications success/error (comme `showToast`)
- `ConfirmDialog.tsx` : (comme `showConfirm`)
- `StatCard.tsx`, `TabBar.tsx`, `DataTable.tsx`

### Phase 7: Intégration Go embed (5 min)
- `vite build` → `dist/`
- Modifier `internal/dashboard/dashboard.go` : `//go:embed` pointe vers `dist/`
- Servir via le même `fs.Sub` handler

### Phase 8: Smoke tests (5 min)
- `vite dev` → tester login, dashboard, projets, tous les onglets
- `vite build` → `go build` → vérifier que le binaire sert bien le dashboard
- Vérifier pas de régression (tous les onglets chargent, modales fonctionnent)

## Total estimé: ~85 min

## Fichiers supprimés après migration
- `internal/dashboard/index.html`
- `internal/dashboard/style.css`
- `internal/dashboard/js/*.js`
- Remplacés par `internal/dashboard/dist/` (embed)

## Risques
- Les 12 tabs de ProjectDetail sont le plus gros boulot — si le temps manque, prioriser les 6 principaux (overview, database, auth, storage, config, api)
- CSS existant a des animations subtiles (fadeSlideIn, closing modal) — à reproduire en Tailwind
- L'API `/admin/projects/:id/usage`, `/analytics`, `/db-analysis` peuvent ne pas exister sur toutes les instances
