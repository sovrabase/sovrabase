package client

import "net/url"

// Integration holds public integration config returned by the API.
type Integration struct {
	ID     string                 `json:"id"`
	Name   string                 `json:"name"`
	Config map[string]interface{} `json:"config"`
}

// GetIntegrations returns enabled integrations with public config only.
func (c *Client) GetIntegrations() ([]Integration, error) {
	var resp struct {
		Integrations []Integration `json:"integrations"`
	}
	if err := c.doJSON("GET", "/api/v1/integrations", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Integrations == nil {
		return []Integration{}, nil
	}
	return resp.Integrations, nil
}

// IntegrationAction executes a server-side integration action.
func (c *Client) IntegrationAction(provider, action string, data map[string]interface{}) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"action": action,
		"data":   data,
	}
	var resp map[string]interface{}
	if err := c.doJSON("POST", "/api/v1/integrations/"+provider+"/action", body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeFunction calls an HTTP-triggered serverless function.
func (c *Client) InvokeFunction(name string, method string, body interface{}) (map[string]interface{}, error) {
	path := "/api/v1/functions/" + url.PathEscape(name)
	var resp map[string]interface{}
	if err := c.doJSON(method, path, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
