package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ─── PayPal ──────────────────────────────────────────────────────────────────

func (s *Server) handlePayPalAction(w http.ResponseWriter, r *http.Request, config map[string]interface{}, action string, data map[string]interface{}) {
	clientID := getString(config, "client_id")
	clientSecret := getString(config, "client_secret")
	sandbox := config["sandbox"] == true

	if clientID == "" || clientSecret == "" {
		writeError(w, http.StatusBadRequest, "PayPal client_id and client_secret are required")
		return
	}

	baseURL := "https://api-m.paypal.com"
	if sandbox {
		baseURL = "https://api-m.sandbox.paypal.com"
	}

	switch action {
	case "get_access_token":
		// Exchange client credentials for an access token
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		req, _ := http.NewRequest("POST", baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
		req.SetBasicAuth(clientID, clientSecret)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "PayPal token request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			writeError(w, resp.StatusCode, "PayPal error: "+string(body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)

	case "create_order":
		// Create a PayPal order. Data should contain: { "amount": "10.00", "currency": "USD", "description": "..." }
		token, err := getPayPalAccessToken(baseURL, clientID, clientSecret)
		if err != nil {
			writeError(w, http.StatusBadGateway, "PayPal auth failed: "+err.Error())
			return
		}

		amount := getString(data, "amount")
		currency := getString(data, "currency")
		if currency == "" {
			currency = "USD"
		}
		description := getString(data, "description")

		orderReq := map[string]interface{}{
			"intent": "CAPTURE",
			"purchase_units": []map[string]interface{}{{
				"amount": map[string]string{
					"currency_code": currency,
					"value":         amount,
				},
				"description": description,
			}},
		}
		reqBody, _ := json.Marshal(orderReq)

		req, _ := http.NewRequest("POST", baseURL+"/v2/checkout/orders", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "PayPal request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode > 299 {
			writeError(w, resp.StatusCode, "PayPal error: "+string(body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)

	case "capture_order":
		// Capture a previously approved order. Data: { "order_id": "..." }
		orderID := getString(data, "order_id")
		if orderID == "" {
			writeError(w, http.StatusBadRequest, "order_id is required")
			return
		}

		token, err := getPayPalAccessToken(baseURL, clientID, clientSecret)
		if err != nil {
			writeError(w, http.StatusBadGateway, "PayPal auth failed: "+err.Error())
			return
		}

		req, _ := http.NewRequest("POST", baseURL+"/v2/checkout/orders/"+orderID+"/capture", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "PayPal request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode > 299 {
			writeError(w, resp.StatusCode, "PayPal error: "+string(body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)

	default:
		writeError(w, http.StatusBadRequest, "Unknown PayPal action: "+action+". Available: get_access_token, create_order, capture_order")
	}
}

func getPayPalAccessToken(baseURL, clientID, clientSecret string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	req, _ := http.NewRequest("POST", baseURL+"/v1/oauth2/token", strings.NewReader(form.Encode()))
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.AccessToken, nil
}

// ─── Stripe ──────────────────────────────────────────────────────────────────

func (s *Server) handleStripeAction(w http.ResponseWriter, r *http.Request, config map[string]interface{}, action string, data map[string]interface{}) {
	secretKey := getString(config, "secret_key")
	if secretKey == "" {
		writeError(w, http.StatusBadRequest, "Stripe secret_key is required")
		return
	}

	switch action {
	case "create_payment_intent":
		amount, ok := data["amount"].(float64)
		if !ok {
			writeError(w, http.StatusBadRequest, "amount (in cents) is required")
			return
		}
		currency := getString(data, "currency")
		if currency == "" {
			currency = "usd"
		}

		form := url.Values{}
		form.Set("amount", fmt.Sprintf("%.0f", amount))
		form.Set("currency", currency)
		form.Set("payment_method_types[]", "card")

		req, _ := http.NewRequest("POST", "https://api.stripe.com/v1/payment_intents", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(secretKey, "")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "Stripe request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode > 299 {
			writeError(w, resp.StatusCode, "Stripe error: "+string(body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)

	case "create_checkout_session":
		form := url.Values{}
		if priceID := getString(data, "price_id"); priceID != "" {
			form.Set("line_items[0][price]", priceID)
			form.Set("line_items[0][quantity]", "1")
		}
		form.Set("mode", getString(data, "mode"))
		if successURL := getString(data, "success_url"); successURL != "" {
			form.Set("success_url", successURL)
		}
		if cancelURL := getString(data, "cancel_url"); cancelURL != "" {
			form.Set("cancel_url", cancelURL)
		}

		req, _ := http.NewRequest("POST", "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(secretKey, "")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "Stripe request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode > 299 {
			writeError(w, resp.StatusCode, "Stripe error: "+string(body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)

	default:
		writeError(w, http.StatusBadRequest, "Unknown Stripe action: "+action+". Available: create_payment_intent, create_checkout_session")
	}
}

// ─── SendGrid ────────────────────────────────────────────────────────────────

func (s *Server) handleSendGridAction(w http.ResponseWriter, r *http.Request, config map[string]interface{}, action string, data map[string]interface{}) {
	apiKey := getString(config, "api_key")
	if apiKey == "" {
		writeError(w, http.StatusBadRequest, "SendGrid api_key is required")
		return
	}

	switch action {
	case "send_email":
		to := getString(data, "to")
		from := getString(config, "from_email")
		fromName := getString(config, "from_name")
		if from == "" {
			from = getString(data, "from")
		}
		subject := getString(data, "subject")
		bodyText := getString(data, "text")
		bodyHTML := getString(data, "html")

		if to == "" || from == "" || subject == "" {
			writeError(w, http.StatusBadRequest, "to, from, and subject are required")
			return
		}

		content := []map[string]string{}
		if bodyText != "" {
			content = append(content, map[string]string{"type": "text/plain", "value": bodyText})
		}
		if bodyHTML != "" {
			content = append(content, map[string]string{"type": "text/html", "value": bodyHTML})
		}
		if len(content) == 0 {
			writeError(w, http.StatusBadRequest, "text or html body is required")
			return
		}

		payload := map[string]interface{}{
			"personalizations": []map[string]interface{}{{
				"to": []map[string]string{{"email": to}},
			}},
			"from": map[string]string{"email": from, "name": fromName},
			"subject":  subject,
			"content":  content,
		}
		reqBody, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "SendGrid request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode > 299 {
			body, _ := io.ReadAll(resp.Body)
			writeError(w, resp.StatusCode, "SendGrid error: "+string(body))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})

	default:
		writeError(w, http.StatusBadRequest, "Unknown SendGrid action: "+action+". Available: send_email")
	}
}

// ─── Discord Webhook ─────────────────────────────────────────────────────────

func (s *Server) handleDiscordWebhookAction(w http.ResponseWriter, r *http.Request, config map[string]interface{}, action string, data map[string]interface{}) {
	webhookURL := getString(config, "webhook_url")
	if webhookURL == "" {
		writeError(w, http.StatusBadRequest, "Discord webhook_url is required")
		return
	}

	switch action {
	case "send_message":
		content := getString(data, "content")
		username := getString(config, "username")
		if username == "" {
			username = getString(data, "username")
		}

		if content == "" {
			writeError(w, http.StatusBadRequest, "content is required")
			return
		}

		payload := map[string]string{"content": content}
		if username != "" {
			payload["username"] = username
		}
		reqBody, _ := json.Marshal(payload)

		resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(reqBody))
		if err != nil {
			writeError(w, http.StatusBadGateway, "Discord webhook failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode > 299 {
			body, _ := io.ReadAll(resp.Body)
			writeError(w, resp.StatusCode, "Discord error: "+string(body))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})

	default:
		writeError(w, http.StatusBadRequest, "Unknown Discord action: "+action+". Available: send_message")
	}
}

// ─── Slack Webhook ───────────────────────────────────────────────────────────

func (s *Server) handleSlackWebhookAction(w http.ResponseWriter, r *http.Request, config map[string]interface{}, action string, data map[string]interface{}) {
	webhookURL := getString(config, "webhook_url")
	if webhookURL == "" {
		writeError(w, http.StatusBadRequest, "Slack webhook_url is required")
		return
	}

	switch action {
	case "send_message":
		text := getString(data, "text")
		if text == "" {
			writeError(w, http.StatusBadRequest, "text is required")
			return
		}

		payload, _ := json.Marshal(map[string]string{"text": text})
		resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(payload))
		if err != nil {
			writeError(w, http.StatusBadGateway, "Slack webhook failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode > 299 {
			body, _ := io.ReadAll(resp.Body)
			writeError(w, resp.StatusCode, "Slack error: "+string(body))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})

	default:
		writeError(w, http.StatusBadRequest, "Unknown Slack action: "+action+". Available: send_message")
	}
}

// ─── Twilio ──────────────────────────────────────────────────────────────────

func (s *Server) handleTwilioAction(w http.ResponseWriter, r *http.Request, config map[string]interface{}, action string, data map[string]interface{}) {
	accountSID := getString(config, "account_sid")
	authToken := getString(config, "auth_token")
	fromNumber := getString(config, "from_number")

	if accountSID == "" || authToken == "" {
		writeError(w, http.StatusBadRequest, "Twilio account_sid and auth_token are required")
		return
	}

	switch action {
	case "send_sms":
		to := getString(data, "to")
		body := getString(data, "body")
		if to == "" || body == "" {
			writeError(w, http.StatusBadRequest, "to and body are required")
			return
		}

		form := url.Values{}
		form.Set("To", to)
		form.Set("From", fromNumber)
		form.Set("Body", body)

		url := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", accountSID)
		req, _ := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(accountSID, authToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeError(w, http.StatusBadGateway, "Twilio request failed: "+err.Error())
			return
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		if resp.StatusCode > 299 {
			writeError(w, resp.StatusCode, "Twilio error: "+string(respBody))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(respBody)

	default:
		writeError(w, http.StatusBadRequest, "Unknown Twilio action: "+action+". Available: send_sms")
	}
}
