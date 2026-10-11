# MCP server

Sovrabase includes a [Model Context Protocol](https://modelcontextprotocol.io) server. AI clients such as Claude Code, Claude Desktop and Cursor can use it to inspect and work with one project: its collections and records, storage, serverless functions and request logs.

- Endpoint: `POST https://HOST/mcp` (Streamable HTTP transport, JSON responses, protocol `2025-06-18`; `2025-03-26` is also accepted)
- Auth: `Authorization: Bearer sbmcp_...`. The token determines which project the client sees.
- MCP is **off by default** for every project. Until the owner enables it, `/mcp` answers `403`.

## 1. Enable MCP for a project

In the dashboard, open the project's MCP settings and switch MCP on. With the admin API:

```http
PUT /admin/projects/{id}/mcp
{"enabled": true, "disabled_tools": ["delete_record"]}
```

Tools listed in `disabled_tools` are hidden from `tools/list` and refused when called.

## 2. Create an access token

```http
POST /admin/projects/{id}/mcp/tokens
{"name": "my laptop", "scopes": ["read", "write"]}
```

The response contains `token` (`sbmcp_...`). It is shown **once**: the server stores only its SHA-256 hash. Revoke a token with `DELETE /admin/projects/{id}/mcp/tokens/{tokenId}`.

| Scope       | Tools |
|-------------|-------|
| `read` (default) | `list_collections`, `describe_collection`, `query_records`, `get_record`, `list_buckets`, `list_files` |
| `write`     | `insert_record`, `update_record`, `delete_record` |
| `schema`    | `create_collection`, `create_index` |
| `functions` | `list_functions`, `invoke_function` |
| `logs`      | `read_logs` |

Scopes do not imply each other. A token that should both read and write records needs `["read", "write"]`.

Safety rules that apply to every token:
- Internal `__*` collections are never exposed.
- System collections whose names start with `_` (for example `_users`) are read-only.
- Sensitive `_users` fields such as password hashes and reset tokens are removed from results.
- `query_records` returns at most 100 records per call.

The last 200 tool calls per project are kept in memory and can be read with `GET /admin/projects/{id}/mcp/calls?limit=50`. The list is cleared when the server restarts.

## 3. Connect a client

### Claude Code

```bash
claude mcp add --transport http sovrabase https://HOST/mcp \
  --header "Authorization: Bearer TOKEN"
```

### Generic JSON config (Claude Desktop, Cursor, others)

For clients that support remote HTTP servers:

```json
{
  "mcpServers": {
    "sovrabase": {
      "type": "http",
      "url": "https://HOST/mcp",
      "headers": {
        "Authorization": "Bearer TOKEN"
      }
    }
  }
}
```

For clients that only launch local (stdio) servers, bridge with `mcp-remote`:

```json
{
  "mcpServers": {
    "sovrabase": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://HOST/mcp", "--header", "Authorization: Bearer TOKEN"]
    }
  }
}
```

## Quick check with curl

```bash
curl -s https://HOST/mcp \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Admin API reference

| Method | Path | Body / response |
|--------|------|-----------------|
| GET | `/admin/projects/{id}/mcp` | `{"enabled","endpoint":"/mcp","disabled_tools":[],"tools":[{"name","description","scope","write","enabled"}]}` |
| PUT | `/admin/projects/{id}/mcp` | body `{"enabled","disabled_tools"}` (either field optional); returns the same shape as GET |
| GET | `/admin/projects/{id}/mcp/tokens` | `[{"id","name","prefix","scopes","created_at","last_used_at"}]` |
| POST | `/admin/projects/{id}/mcp/tokens` | body `{"name","scopes"}`; returns `201` with `{"token", ...token fields}` |
| DELETE | `/admin/projects/{id}/mcp/tokens/{tokenId}` | `204` |
| GET | `/admin/projects/{id}/mcp/calls?limit=50` | `[{"at","token_name","tool","ok","duration_ms","error"}]`, newest first, max 200 |
