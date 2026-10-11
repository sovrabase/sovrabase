# Sovrabase UI kit

Everything in the dashboard is built from `src/ui` (import from `'../../ui'`) and the shell in `src/shell`.
Do not write raw `<input>`, `<select>`, `<textarea>` or `confirm()` in pages.

## Tokens (Tailwind classes)

Light first, dark comes automatically. Use only these colour names:

| Use | Class |
|---|---|
| Page background | `bg-bg` |
| Cards, sheets | `bg-surface` |
| Wells, table heads, read-only | `bg-sunken` |
| Row hover | `hover:bg-hover` |
| Borders | `border-line` (default), `border-line-2` (controls) |
| Text | `text-ink` (primary), `text-ink-2` (body), `text-ink-3` (hints, meta) |
| Accent | `text-accent`, `bg-accent`, `bg-accent-soft`, `text-accent-2`, `text-on-accent` |
| Status | `ok / ok-soft`, `warn / warn-soft`, `danger / danger-soft` |
| Code panels | `bg-code text-code-ink` |

Never use raw hex, `text-white` on surfaces, `bg-black`, glows, gradients or glass. Radii: `rounded-md` (8px controls),
`rounded-lg` (12px cards). Fonts: Instrument Sans (default) and `font-mono` (JetBrains Mono) for ids, keys, code, paths.
Type scale: page title 22px semibold (PageHeader), section 15-16px semibold, body 14px, meta 12.5px.

## Page anatomy

```tsx
<Page>
  <PageHeader title="Storage" description="One line on what this page is for." actions={<Button variant="primary">New bucket</Button>} />
  <Card padded={false}> <CardHeader title=… /> <DataTable … /> </Card>
</Page>
```

* Lists: `Card padded={false}` + `DataTable` (turns into cards on phones) or `Row`s.
* Create / edit: a `Sheet` (side panel) with the form inside and Cancel / primary button in `footer`.
* Settings pages: `Section` blocks (title left, controls right) + `useDraft` + `SaveBar` (sticky save, only when dirty).
* Destructive actions: `ConfirmDialog` with `confirmText={name}` for anything that can't be undone (project, collection, bucket, user).
* Empty lists: `EmptyState` with one primary action. Loading: `Spinner` or `Skeleton`. Errors: `Banner tone="danger"`.
* Feedback: `useToast().showToast(msg, 'success' | 'error')` for results of actions.
* In-page sub-navigation: `Tabs` (keep it in the URL when the route has a `:section` param).
* Filters: `SearchInput` + `Segmented` in one row above the list.

## Inputs

Every control goes inside `<Field label hint error optional env>`; the Field wires id, aria and the ENV lock.

| Need | Use |
|---|---|
| Text, URL, number | `Input` (`mono` for ids/keys/URLs, `prefix`/`suffix`) |
| Long text | `Textarea` (`mono` for JSON/code) |
| One of many | `Select options={[{value,label}]}` or `Segmented` for 2-4 options |
| Several of a fixed set (events, scopes, methods) | `ChipSelect` |
| Free list (origins, tags, emails) — never comma-separated text | `TokenInput` |
| On/off | `Switch` (with `label` + `description` for a full row) |
| Secret (API key, password, token) — write-only | `SecretInput isSet={…}` |
| Headers, env vars, metadata | `KeyValueEditor` + `kvFromRecord` / `kvToRecord` |
| Cron | `ScheduleInput` (presets + plain-language reading) |
| A settings schema (plugins, integrations) | `SchemaForm fields={…}` + `validateSchema` |
| Copy a value | `CopyButton text=…` |

## Project pages

`const { project, projectId, reload } = useProject()` from `src/shell/ProjectContext`. Build links with
`projectPath(projectId, 'database/orders')` from `src/shell/nav`. API calls: `api<T>(adminProjectPath(projectId, '/…'))`.
