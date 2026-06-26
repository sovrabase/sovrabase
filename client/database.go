package client

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ─── CRUD ──────────────────────────────────────────────────────────────────────

// Insert creates a new document in a collection. Returns the created document.
func (c *Client) Insert(collection string, data Document) (Document, error) {
	var doc Document
	path := "/api/v1/collections/" + url.PathEscape(collection)
	if err := c.doJSON("POST", path, data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// Get retrieves a single document by ID.
func (c *Client) Get(collection, id string) (Document, error) {
	var doc Document
	path := fmt.Sprintf("/api/v1/collections/%s/%s", url.PathEscape(collection), url.PathEscape(id))
	if err := c.doJSON("GET", path, nil, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// Update modifies an existing document. Only the fields present in data are updated.
func (c *Client) Update(collection, id string, data Document) (Document, error) {
	var doc Document
	path := fmt.Sprintf("/api/v1/collections/%s/%s", url.PathEscape(collection), url.PathEscape(id))
	if err := c.doJSON("PUT", path, data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// Delete removes a document by ID.
func (c *Client) Delete(collection, id string) error {
	path := fmt.Sprintf("/api/v1/collections/%s/%s", url.PathEscape(collection), url.PathEscape(id))
	return c.doJSON("DELETE", path, nil, nil)
}

// Patch partially updates an existing document. Only the fields present in data are updated.
func (c *Client) Patch(collection, id string, data Document) (Document, error) {
	var doc Document
	path := fmt.Sprintf("/api/v1/collections/%s/%s", url.PathEscape(collection), url.PathEscape(id))
	if err := c.doJSON("PATCH", path, data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// ─── List ──────────────────────────────────────────────────────────────────────

// List returns documents from a collection with optional pagination and field selection.
func (c *Client) List(collection string, opts *ListOptions) (*ListResponse, error) {
	params := url.Values{}
	if opts != nil {
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			params.Set("offset", strconv.Itoa(opts.Offset))
		}
		if len(opts.Select) > 0 {
			params.Set("select", strings.Join(opts.Select, ","))
		}
	}

	path := "/api/v1/collections/" + url.PathEscape(collection)
	path = buildQueryPath(path, params)

	// List returns either a plain array (no pagination) or {data, total, limit, offset}.
	// We always parse as ListResponse first; if Data is empty, try as array.
	var resp ListResponse
	if err := c.doJSON("GET", path, nil, &resp); err != nil {
		return nil, err
	}

	// If the server returned a plain array, it was decoded as an empty ListResponse.
	if resp.Data == nil && resp.Total == 0 {
		// Try decoding as an array.
		var docs []Document
		path = "/api/v1/collections/" + url.PathEscape(collection)
		path = buildQueryPath(path, params)
		if err := c.doJSON("GET", path, nil, &docs); err != nil {
			return nil, err
		}
		resp.Data = docs
		resp.Total = int64(len(docs))
	}

	if resp.Data == nil {
		resp.Data = []Document{}
	}
	return &resp, nil
}

// ─── Query ─────────────────────────────────────────────────────────────────────

// Query performs a filtered query against a collection.
func (c *Client) Query(collection string, filter Filter, opts *QueryOptions) (*ListResponse, error) {
	body := map[string]interface{}{}
	if filter != nil {
		body["filter"] = filter
	}
	if opts != nil {
		if len(opts.Select) > 0 {
			body["select"] = opts.Select
		}
		if opts.Limit > 0 {
			body["limit"] = opts.Limit
		}
		if opts.Offset > 0 {
			body["offset"] = opts.Offset
		}
		// If QueryOptions has its own Filter, merge it in.
		if opts.Filter != nil {
			body["filter"] = opts.Filter
		}
	}

	path := "/api/v1/collections/" + url.PathEscape(collection) + "/query"

	// Query returns either a plain array or {data, total, limit, offset}.
	var resp ListResponse
	if err := c.doJSON("POST", path, body, &resp); err != nil {
		return nil, err
	}

	if resp.Data == nil && resp.Total == 0 {
		var docs []Document
		if err := c.doJSON("POST", path, body, &docs); err != nil {
			return nil, err
		}
		resp.Data = docs
		resp.Total = int64(len(docs))
	}

	if resp.Data == nil {
		resp.Data = []Document{}
	}
	return &resp, nil
}

// ─── Search ────────────────────────────────────────────────────────────────────

// Search performs a full-text search in a collection.
func (c *Client) Search(collection string, query string, opts *SearchOptions) (*SearchResponse, error) {
	body := map[string]interface{}{
		"query": query,
	}
	if opts != nil {
		if len(opts.Fields) > 0 {
			body["fields"] = opts.Fields
		}
		if opts.Limit > 0 {
			body["limit"] = opts.Limit
		}
	}

	path := "/api/v1/collections/" + url.PathEscape(collection) + "/search"
	var resp SearchResponse
	if err := c.doJSON("POST", path, body, &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil {
		resp.Data = []Document{}
	}
	return &resp, nil
}

// ─── Batch ─────────────────────────────────────────────────────────────────────

// Batch executes multiple operations (insert, update, delete) in a single request.
func (c *Client) Batch(collection string, ops []BatchOp) (*BatchResponse, error) {
	body := map[string]interface{}{
		"operations": ops,
	}

	path := "/api/v1/collections/" + url.PathEscape(collection) + "/batch"
	var resp BatchResponse
	if err := c.doJSON("POST", path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
