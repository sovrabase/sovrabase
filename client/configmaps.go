package client

import (
	"fmt"
)

// ConfigEntry represents a remote config key-value pair.
type ConfigEntry struct {
	Key         string      `json:"key"`
	Value       interface{} `json:"value"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Public      bool        `json:"public"`
}

// ConfigGetAll retrieves all remote config entries for the project.
func (c *Client) ConfigGetAll() ([]ConfigEntry, error) {
	var resp struct {
		Data  []ConfigEntry `json:"data"`
		Count int           `json:"count"`
	}
	if err := c.doJSON("GET", "/api/v1/config/", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// ConfigGet retrieves a single remote config entry by key.
func (c *Client) ConfigGet(key string) (*ConfigEntry, error) {
	var entry ConfigEntry
	if err := c.doJSON("GET", "/api/v1/config/"+key, nil, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ConfigSet creates or updates a remote config entry.
func (c *Client) ConfigSet(key string, value interface{}, configType, description string, public bool) (*ConfigEntry, error) {
	body := map[string]interface{}{
		"key":         key,
		"value":       value,
		"type":        configType,
		"description": description,
		"public":      public,
	}
	var entry ConfigEntry
	if err := c.doJSON("POST", "/api/v1/config/", body, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// ConfigDelete removes a remote config entry.
func (c *Client) ConfigDelete(key string) error {
	return c.doJSON("DELETE", fmt.Sprintf("/api/v1/config/%s", key), nil, nil)
}

// ConfigGetPublic retrieves all public config entries as a key→value map.
// This endpoint does not require authentication.
func (c *Client) ConfigGetPublic() (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := c.doJSON("GET", "/api/v1/config/public", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}
