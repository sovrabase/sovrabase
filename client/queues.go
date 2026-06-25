package client

// ─── Queues ──────────────────────────────────────────────────────────────

// QueueMessage is a message received from a queue.
type QueueMessage struct {
	ID    string                 `json:"id"`
	Body  map[string]interface{} `json:"body"`
	Queue string                 `json:"queue"`
}

// QueueSend sends a message to a named queue.
func (c *Client) QueueSend(queueName string, body map[string]interface{}) (string, error) {
	req := map[string]interface{}{
		"queue": queueName,
		"body":  body,
	}
	var resp struct {
		ID    string `json:"id"`
		Queue string `json:"queue"`
	}
	if err := c.doJSON("POST", "/api/v1/queues/send", req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// QueueReceive retrieves messages from a queue. Use limit=0 for default.
func (c *Client) QueueReceive(queueName string, limit int) ([]QueueMessage, error) {
	req := map[string]interface{}{
		"queue": queueName,
		"limit": limit,
	}
	var resp struct {
		Data  []QueueMessage `json:"data"`
		Count int            `json:"count"`
	}
	if err := c.doJSON("POST", "/api/v1/queues/receive", req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// QueueDelete deletes a message from a queue by ID.
func (c *Client) QueueDelete(queueName, msgID string) error {
	req := map[string]interface{}{
		"queue": queueName,
		"id":    msgID,
	}
	return c.doJSON("POST", "/api/v1/queues/delete", req, nil)
}
