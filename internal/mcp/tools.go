package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// IndexInfo describes a secondary index.
type IndexInfo struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

// FunctionInfo describes a deployed serverless function (code omitted).
type FunctionInfo struct {
	Name       string `json:"name"`
	Language   string `json:"language,omitempty"`
	Trigger    string `json:"trigger"`
	Collection string `json:"collection,omitempty"`
	Schedule   string `json:"schedule,omitempty"`
	Public     bool   `json:"public"`
	Enabled    bool   `json:"enabled"`
}

// Backend is what the MCP tools operate on for a single project. The api
// package implements it on top of the same stores the admin API uses.
type Backend interface {
	ListCollections() ([]string, error)
	CountRecords(collection string) (int64, error)
	CreateCollection(name string) error
	// QueryRecords returns matching records. limit <= 0 means no limit.
	QueryRecords(collection string, filter map[string]interface{}, limit, offset int) ([]map[string]interface{}, error)
	// GetRecord returns (nil, nil) when the record does not exist.
	GetRecord(collection, id string) (map[string]interface{}, error)
	InsertRecord(collection, id string, doc map[string]interface{}) error
	UpdateRecord(collection, id string, doc map[string]interface{}) error
	DeleteRecord(collection, id string) error
	ListIndexes(collection string) ([]IndexInfo, error)
	CreateIndex(collection, field string, unique bool) error
	GetRules(collection string) (interface{}, error)
	ListFunctions() ([]FunctionInfo, error)
	InvokeFunction(ctx context.Context, name string, payload map[string]interface{}) (interface{}, error)
	ListBuckets(ctx context.Context) ([]string, error)
	ListFiles(ctx context.Context, bucket, prefix string) (interface{}, error)
	// ReadLogs returns up to max request-log entries, newest first.
	ReadLogs(max int) ([]map[string]interface{}, error)
}

// Tool is one MCP tool exposed to clients.
type Tool struct {
	Name        string
	Title       string
	Description string
	Scope       string
	Write       bool
	Destructive bool
	InputSchema map[string]interface{}
	run         func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error)
}

// toolError is a failure reported to the model as an isError tool result.
type toolError struct{ msg string }

func (e *toolError) Error() string { return e.msg }

func errf(format string, a ...interface{}) error { return &toolError{fmt.Sprintf(format, a...)} }

const (
	maxQueryLimit     = 100
	defaultQueryLimit = 20
	describeSample    = 50
	maxLogLimit       = 200
	defaultLogLimit   = 50
	logScanWindow     = 2000
)

var sensitiveUserFields = []string{"password_hash", "verification_token", "reset_token", "magic_link_token", "mfa_secret", "mfa_recovery_codes"}

// ── argument helpers ───────────────────────────────────────────────────────

func strArg(args map[string]interface{}, key string, required bool) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		if required {
			return "", errf("missing required argument %q", key)
		}
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", errf("argument %q must be a string", key)
	}
	s = strings.TrimSpace(s)
	if required && s == "" {
		return "", errf("argument %q must not be empty", key)
	}
	return s, nil
}

func intArg(args map[string]interface{}, key string, def int) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return def, nil
	}
	f, ok := v.(float64)
	if !ok {
		return 0, errf("argument %q must be a number", key)
	}
	return int(f), nil
}

func objArg(args map[string]interface{}, key string, required bool) (map[string]interface{}, error) {
	v, ok := args[key]
	if !ok || v == nil {
		if required {
			return nil, errf("missing required argument %q", key)
		}
		return nil, nil
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, errf("argument %q must be an object", key)
	}
	return m, nil
}

// collectionArg reads "collection" and enforces visibility rules: internal
// "__" collections are never reachable; "_" system collections are
// read-only over MCP.
func collectionArg(args map[string]interface{}, write bool) (string, error) {
	c, err := strArg(args, "collection", true)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(c, "__") {
		return "", errf("collection %q is internal and not available over MCP", c)
	}
	if write && strings.HasPrefix(c, "_") {
		return "", errf("collection %q is a system collection and is read-only over MCP", c)
	}
	return c, nil
}

func sanitize(collection string, docs ...map[string]interface{}) {
	if collection != "_users" {
		return
	}
	for _, d := range docs {
		for _, k := range sensitiveUserFields {
			delete(d, k)
		}
	}
}

func stripSystemFields(doc map[string]interface{}) {
	delete(doc, "_id")
	delete(doc, "_createdAt")
	delete(doc, "_updatedAt")
}

func compareValues(a, b interface{}) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	af, aNum := toFloat(a)
	bf, bNum := toFloat(b)
	if aNum && bNum {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		}
		return 0
	}
	as, bs := fmt.Sprint(a), fmt.Sprint(b)
	return strings.Compare(as, bs)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func typeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	}
	if _, ok := toFloat(v); ok {
		return "number"
	}
	if _, ok := v.(fmt.Stringer); ok {
		return "string"
	}
	return fmt.Sprintf("%T", v)
}

// ── schemas ────────────────────────────────────────────────────────────────

func schema(props map[string]interface{}, required ...string) map[string]interface{} {
	s := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func prop(typ, desc string) map[string]interface{} {
	return map[string]interface{}{"type": typ, "description": desc}
}

var collectionProp = prop("string", "Collection name.")

// ── tool registry ──────────────────────────────────────────────────────────

// Tools returns every tool the server knows, in a stable order.
func Tools() []*Tool { return toolList }

// ToolByName looks up a tool.
func ToolByName(name string) (*Tool, bool) {
	t, ok := toolIndex[name]
	return t, ok
}

var toolIndex map[string]*Tool

func init() {
	toolIndex = make(map[string]*Tool, len(toolList))
	for _, t := range toolList {
		toolIndex[t.Name] = t
	}
}

var toolList = []*Tool{
	{
		Name:        "list_collections",
		Title:       "List collections",
		Description: "List the project's database collections with their record counts.",
		Scope:       ScopeRead,
		InputSchema: schema(map[string]interface{}{}),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			cols, err := b.ListCollections()
			if err != nil {
				return nil, err
			}
			out := make([]map[string]interface{}, 0, len(cols))
			for _, c := range cols {
				if strings.HasPrefix(c, "__") {
					continue
				}
				n, _ := b.CountRecords(c)
				out = append(out, map[string]interface{}{"name": c, "count": n})
			}
			return map[string]interface{}{"collections": out}, nil
		},
	},
	{
		Name:        "describe_collection",
		Title:       "Describe collection",
		Description: "Describe a collection: record count, fields inferred from a sample of records (with observed types), indexes and access rules.",
		Scope:       ScopeRead,
		InputSchema: schema(map[string]interface{}{"collection": collectionProp}, "collection"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, false)
			if err != nil {
				return nil, err
			}
			docs, err := b.QueryRecords(c, nil, describeSample, 0)
			if err != nil {
				return nil, err
			}
			sanitize(c, docs...)
			type fieldStat struct {
				types map[string]bool
				count int
			}
			stats := map[string]*fieldStat{}
			for _, d := range docs {
				for k, v := range d {
					fs, ok := stats[k]
					if !ok {
						fs = &fieldStat{types: map[string]bool{}}
						stats[k] = fs
					}
					fs.count++
					fs.types[typeName(v)] = true
				}
			}
			names := make([]string, 0, len(stats))
			for k := range stats {
				names = append(names, k)
			}
			sort.Strings(names)
			fields := make([]map[string]interface{}, 0, len(names))
			for _, k := range names {
				types := make([]string, 0, len(stats[k].types))
				for t := range stats[k].types {
					types = append(types, t)
				}
				sort.Strings(types)
				fields = append(fields, map[string]interface{}{
					"name":       k,
					"types":      types,
					"present_in": stats[k].count,
				})
			}
			count, _ := b.CountRecords(c)
			idx, err := b.ListIndexes(c)
			if err != nil || idx == nil {
				idx = []IndexInfo{}
			}
			rules, _ := b.GetRules(c)
			return map[string]interface{}{
				"collection":  c,
				"count":       count,
				"sample_size": len(docs),
				"fields":      fields,
				"indexes":     idx,
				"rules":       rules,
			}, nil
		},
	},
	{
		Name:  "query_records",
		Title: "Query records",
		Description: "Query records in a collection. `filter` is an object of field conditions, either equality ({\"status\":\"active\"}) " +
			"or operators ({\"age\":{\"$gt\":18}}; supported: $eq $ne $gt $gte $lt $lte $in $nin $contains $startsWith $exists $regex). " +
			"Optional `sort` field with `order` asc|desc, `limit` (default 20, max 100) and `offset`.",
		Scope: ScopeRead,
		InputSchema: schema(map[string]interface{}{
			"collection": collectionProp,
			"filter":     prop("object", "Field conditions to match."),
			"sort":       prop("string", "Field to sort by."),
			"order":      map[string]interface{}{"type": "string", "enum": []string{"asc", "desc"}, "description": "Sort order (default asc)."},
			"limit":      map[string]interface{}{"type": "integer", "minimum": 1, "maximum": maxQueryLimit, "description": "Max records to return (default 20, max 100)."},
			"offset":     map[string]interface{}{"type": "integer", "minimum": 0, "description": "Records to skip."},
		}, "collection"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, false)
			if err != nil {
				return nil, err
			}
			filter, err := objArg(args, "filter", false)
			if err != nil {
				return nil, err
			}
			sortField, err := strArg(args, "sort", false)
			if err != nil {
				return nil, err
			}
			order, err := strArg(args, "order", false)
			if err != nil {
				return nil, err
			}
			if order != "" && order != "asc" && order != "desc" {
				return nil, errf("order must be \"asc\" or \"desc\"")
			}
			limit, err := intArg(args, "limit", defaultQueryLimit)
			if err != nil {
				return nil, err
			}
			if limit <= 0 {
				limit = defaultQueryLimit
			}
			if limit > maxQueryLimit {
				limit = maxQueryLimit
			}
			offset, err := intArg(args, "offset", 0)
			if err != nil {
				return nil, err
			}
			if offset < 0 {
				offset = 0
			}
			var docs []map[string]interface{}
			if sortField == "" {
				docs, err = b.QueryRecords(c, filter, limit, offset)
				if err != nil {
					return nil, err
				}
			} else {
				all, err := b.QueryRecords(c, filter, 0, 0)
				if err != nil {
					return nil, err
				}
				sort.SliceStable(all, func(i, j int) bool {
					cmp := compareValues(all[i][sortField], all[j][sortField])
					if order == "desc" {
						return cmp > 0
					}
					return cmp < 0
				})
				if offset < len(all) {
					all = all[offset:]
				} else {
					all = nil
				}
				if len(all) > limit {
					all = all[:limit]
				}
				docs = all
			}
			if docs == nil {
				docs = []map[string]interface{}{}
			}
			sanitize(c, docs...)
			return map[string]interface{}{"records": docs, "count": len(docs), "limit": limit, "offset": offset}, nil
		},
	},
	{
		Name:        "get_record",
		Title:       "Get record",
		Description: "Fetch a single record by its ID.",
		Scope:       ScopeRead,
		InputSchema: schema(map[string]interface{}{"collection": collectionProp, "id": prop("string", "Record ID (_id).")}, "collection", "id"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, false)
			if err != nil {
				return nil, err
			}
			id, err := strArg(args, "id", true)
			if err != nil {
				return nil, err
			}
			doc, err := b.GetRecord(c, id)
			if err != nil {
				return nil, err
			}
			if doc == nil {
				return nil, errf("record %q not found in %q", id, c)
			}
			sanitize(c, doc)
			return doc, nil
		},
	},
	{
		Name:        "insert_record",
		Title:       "Insert record",
		Description: "Insert a new record into a collection. Optional `id`; one is generated when omitted. Returns the stored record.",
		Scope:       ScopeWrite,
		Write:       true,
		InputSchema: schema(map[string]interface{}{
			"collection": collectionProp,
			"record":     prop("object", "Record fields."),
			"id":         prop("string", "Optional record ID."),
		}, "collection", "record"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, true)
			if err != nil {
				return nil, err
			}
			rec, err := objArg(args, "record", true)
			if err != nil {
				return nil, err
			}
			id, err := strArg(args, "id", false)
			if err != nil {
				return nil, err
			}
			if id == "" {
				id, _ = rec["_id"].(string)
			}
			stripSystemFields(rec)
			if id != "" {
				if existing, err := b.GetRecord(c, id); err == nil && existing != nil {
					return nil, errf("record %q already exists in %q", id, c)
				}
			}
			if err := b.InsertRecord(c, id, rec); err != nil {
				return nil, err
			}
			return rec, nil
		},
	},
	{
		Name:        "update_record",
		Title:       "Update record",
		Description: "Update a record. Fields in `changes` are merged into the existing record; set `replace` to true to overwrite it entirely. A field set to null is removed when merging.",
		Scope:       ScopeWrite,
		Write:       true,
		InputSchema: schema(map[string]interface{}{
			"collection": collectionProp,
			"id":         prop("string", "Record ID."),
			"changes":    prop("object", "Fields to set."),
			"replace":    prop("boolean", "Replace the whole record instead of merging (default false)."),
		}, "collection", "id", "changes"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, true)
			if err != nil {
				return nil, err
			}
			id, err := strArg(args, "id", true)
			if err != nil {
				return nil, err
			}
			changes, err := objArg(args, "changes", true)
			if err != nil {
				return nil, err
			}
			replace, _ := args["replace"].(bool)
			existing, err := b.GetRecord(c, id)
			if err != nil {
				return nil, err
			}
			if existing == nil {
				return nil, errf("record %q not found in %q", id, c)
			}
			stripSystemFields(changes)
			doc := changes
			if !replace {
				doc = existing
				for k, v := range changes {
					if v == nil {
						delete(doc, k)
					} else {
						doc[k] = v
					}
				}
				delete(doc, "_id")
				delete(doc, "_updatedAt")
			}
			if err := b.UpdateRecord(c, id, doc); err != nil {
				return nil, err
			}
			return doc, nil
		},
	},
	{
		Name:        "delete_record",
		Title:       "Delete record",
		Description: "Delete a record by ID. This cannot be undone.",
		Scope:       ScopeWrite,
		Write:       true,
		Destructive: true,
		InputSchema: schema(map[string]interface{}{"collection": collectionProp, "id": prop("string", "Record ID.")}, "collection", "id"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, true)
			if err != nil {
				return nil, err
			}
			id, err := strArg(args, "id", true)
			if err != nil {
				return nil, err
			}
			if existing, err := b.GetRecord(c, id); err == nil && existing == nil {
				return nil, errf("record %q not found in %q", id, c)
			}
			if err := b.DeleteRecord(c, id); err != nil {
				return nil, err
			}
			return map[string]interface{}{"deleted": true, "collection": c, "id": id}, nil
		},
	},
	{
		Name:        "create_collection",
		Title:       "Create collection",
		Description: "Create a new, empty collection.",
		Scope:       ScopeSchema,
		Write:       true,
		InputSchema: schema(map[string]interface{}{"name": prop("string", "Collection name (letters, digits, '-' and '_'; must not start with '_').")}, "name"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			name, err := strArg(args, "name", true)
			if err != nil {
				return nil, err
			}
			if strings.HasPrefix(name, "_") {
				return nil, errf("collection names starting with '_' are reserved")
			}
			for _, r := range name {
				if !(r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
					return nil, errf("collection name may only contain letters, digits, '-' and '_'")
				}
			}
			if err := b.CreateCollection(name); err != nil {
				return nil, err
			}
			return map[string]interface{}{"created": true, "collection": name}, nil
		},
	},
	{
		Name:        "create_index",
		Title:       "Create index",
		Description: "Create a secondary index on a collection field to speed up equality filters. Set `unique` to enforce uniqueness.",
		Scope:       ScopeSchema,
		Write:       true,
		InputSchema: schema(map[string]interface{}{
			"collection": collectionProp,
			"field":      prop("string", "Field to index."),
			"unique":     prop("boolean", "Enforce unique values (default false)."),
		}, "collection", "field"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			c, err := collectionArg(args, true)
			if err != nil {
				return nil, err
			}
			field, err := strArg(args, "field", true)
			if err != nil {
				return nil, err
			}
			unique, _ := args["unique"].(bool)
			if err := b.CreateIndex(c, field, unique); err != nil {
				return nil, err
			}
			typ := "simple"
			if unique {
				typ = "unique"
			}
			return map[string]interface{}{"created": true, "collection": c, "index": IndexInfo{Field: field, Type: typ}}, nil
		},
	},
	{
		Name:        "list_functions",
		Title:       "List functions",
		Description: "List the project's serverless functions with their trigger, schedule and status.",
		Scope:       ScopeFunctions,
		InputSchema: schema(map[string]interface{}{}),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			fns, err := b.ListFunctions()
			if err != nil {
				return nil, err
			}
			if fns == nil {
				fns = []FunctionInfo{}
			}
			return map[string]interface{}{"functions": fns}, nil
		},
	},
	{
		Name:        "invoke_function",
		Title:       "Invoke function",
		Description: "Run a serverless function by name. For HTTP functions `payload` is sent as the request body; for event functions it is the event data. Returns status, output, response and duration.",
		Scope:       ScopeFunctions,
		Write:       true,
		InputSchema: schema(map[string]interface{}{
			"name":    prop("string", "Function name."),
			"payload": prop("object", "Request body or event data."),
		}, "name"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			name, err := strArg(args, "name", true)
			if err != nil {
				return nil, err
			}
			payload, err := objArg(args, "payload", false)
			if err != nil {
				return nil, err
			}
			return b.InvokeFunction(ctx, name, payload)
		},
	},
	{
		Name:        "list_buckets",
		Title:       "List storage buckets",
		Description: "List the project's storage buckets.",
		Scope:       ScopeRead,
		InputSchema: schema(map[string]interface{}{}),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			buckets, err := b.ListBuckets(ctx)
			if err != nil {
				return nil, err
			}
			if buckets == nil {
				buckets = []string{}
			}
			return map[string]interface{}{"buckets": buckets}, nil
		},
	},
	{
		Name:        "list_files",
		Title:       "List files",
		Description: "List files in a storage bucket, optionally under a path prefix.",
		Scope:       ScopeRead,
		InputSchema: schema(map[string]interface{}{
			"bucket": prop("string", "Bucket name."),
			"prefix": prop("string", "Optional path prefix."),
		}, "bucket"),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			bucket, err := strArg(args, "bucket", true)
			if err != nil {
				return nil, err
			}
			prefix, err := strArg(args, "prefix", false)
			if err != nil {
				return nil, err
			}
			files, err := b.ListFiles(ctx, bucket, prefix)
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{"bucket": bucket, "files": files}, nil
		},
	},
	{
		Name:  "read_logs",
		Title: "Read request logs",
		Description: "Read the project's most recent request logs, newest first. `level` filters by minimum severity: " +
			"info (all), warn (HTTP status >= 400) or error (status >= 500). `limit` defaults to 50, max 200.",
		Scope: ScopeLogs,
		InputSchema: schema(map[string]interface{}{
			"limit": map[string]interface{}{"type": "integer", "minimum": 1, "maximum": maxLogLimit, "description": "Max entries (default 50, max 200)."},
			"level": map[string]interface{}{"type": "string", "enum": []string{"info", "warn", "error"}, "description": "Minimum severity."},
		}),
		run: func(ctx context.Context, b Backend, args map[string]interface{}) (interface{}, error) {
			limit, err := intArg(args, "limit", defaultLogLimit)
			if err != nil {
				return nil, err
			}
			if limit <= 0 {
				limit = defaultLogLimit
			}
			if limit > maxLogLimit {
				limit = maxLogLimit
			}
			level, err := strArg(args, "level", false)
			if err != nil {
				return nil, err
			}
			minRank := 0
			switch strings.ToLower(level) {
			case "", "info":
			case "warn", "warning":
				minRank = 1
			case "error":
				minRank = 2
			default:
				return nil, errf("level must be one of info, warn, error")
			}
			entries, err := b.ReadLogs(logScanWindow)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]interface{}, 0, limit)
			for _, e := range entries {
				lvl, rank := logLevel(e)
				if rank < minRank {
					continue
				}
				e["level"] = lvl
				out = append(out, e)
				if len(out) >= limit {
					break
				}
			}
			return map[string]interface{}{"entries": out, "count": len(out)}, nil
		},
	},
}

func logLevel(e map[string]interface{}) (string, int) {
	status, _ := toFloat(e["status"])
	switch {
	case status >= 500:
		return "error", 2
	case status >= 400:
		return "warn", 1
	}
	return "info", 0
}
