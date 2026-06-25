package client

// ─── Analytics Events ────────────────────────────────────────────────────

// AnalyticsEvent is a single analytics event to ingest.
type AnalyticsEvent struct {
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Timestamp  string                 `json:"timestamp,omitempty"`
}

// IngestEvents sends a batch of analytics events to the server.
func (c *Client) IngestEvents(events []AnalyticsEvent) error {
	req := map[string]interface{}{
		"events": events,
	}
	return c.doJSON("POST", "/api/v1/events", req, nil)
}
