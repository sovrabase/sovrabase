package integrations

// ConfigFieldDef describes a single configuration field for an integration.
type ConfigFieldDef struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // "text", "password", "boolean", "number", "url"
	Required    bool   `json:"required"`
	Placeholder string `json:"placeholder,omitempty"`
	HelpText    string `json:"help_text,omitempty"`
}

// IntegrationDef describes a concrete integration available to projects.
type IntegrationDef struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Description      string           `json:"description"`
	Category         string           `json:"category"`          // payments, email, sms, notifications, search, analytics
	Icon             string           `json:"icon"`              // emoji or single char for badge
	Color            string           `json:"color"`             // hex color for badge
	ConfigFields     []ConfigFieldDef `json:"config_fields"`
	SupportsTriggers bool             `json:"supports_triggers"` // true = server auto-fires on DB/auth events
}

// Catalog is the static list of all available integrations.
var Catalog = []IntegrationDef{
	{
		ID:          "paypal",
		Name:        "PayPal",
		Description: "Accept payments via PayPal. Configure webhooks to track payment events.",
		Category:    "payments",
		Icon:        "P",
		Color:      "#003087",
		ConfigFields: []ConfigFieldDef{
			{Key: "client_id", Label: "Client ID", Type: "text", Required: true, Placeholder: "AeA_..."},
			{Key: "client_secret", Label: "Client Secret", Type: "password", Required: true},
			{Key: "webhook_id", Label: "Webhook ID", Type: "text", HelpText: "PayPal webhook ID for event verification"},
			{Key: "sandbox", Label: "Sandbox Mode", Type: "boolean"},
		},
	},
	{
		ID:          "stripe",
		Name:        "Stripe",
		Description: "Process payments and subscriptions with Stripe.",
		Category:    "payments",
		Icon:        "S",
		Color:       "#635bff",
		ConfigFields: []ConfigFieldDef{
			{Key: "publishable_key", Label: "Publishable Key", Type: "text", Required: true, Placeholder: "pk_live_..."},
			{Key: "secret_key", Label: "Secret Key", Type: "password", Required: true},
			{Key: "webhook_secret", Label: "Webhook Secret", Type: "password", HelpText: "Used to verify Stripe webhook signatures"},
		},
	},
	{
		ID:          "sendgrid",
		Name:        "SendGrid",
		Description: "Send transactional and marketing emails via SendGrid.",
		Category:    "email",
		Icon:        "G",
		Color:       "#1a82e2",
		ConfigFields: []ConfigFieldDef{
			{Key: "api_key", Label: "API Key", Type: "password", Required: true},
			{Key: "from_email", Label: "From Email", Type: "text", Required: true, Placeholder: "noreply@example.com"},
			{Key: "from_name", Label: "From Name", Type: "text", Placeholder: "My App"},
		},
	},
	{
		ID:          "twilio",
		Name:        "Twilio",
		Description: "Send SMS messages and verify phone numbers via Twilio.",
		Category:    "sms",
		Icon:        "T",
		Color:       "#f22f46",
		ConfigFields: []ConfigFieldDef{
			{Key: "account_sid", Label: "Account SID", Type: "text", Required: true},
			{Key: "auth_token", Label: "Auth Token", Type: "password", Required: true},
			{Key: "from_number", Label: "From Number", Type: "text", Required: true, Placeholder: "+1234567890"},
		},
	},
	{
		ID:               "discord_webhook",
		Name:             "Discord Webhooks",
		Description:      "Send notifications to Discord channels via webhooks.",
		Category:         "notifications",
		Icon:             "D",
		Color:            "#5865f2",
		SupportsTriggers: true,
		ConfigFields: []ConfigFieldDef{
			{Key: "webhook_url", Label: "Webhook URL", Type: "url", Required: true, Placeholder: "https://discord.com/api/webhooks/..."},
			{Key: "username", Label: "Bot Username", Type: "text", Placeholder: "Sovrabase Bot"},
		},
	},
	{
		ID:               "slack_webhook",
		Name:             "Slack Webhooks",
		Description:      "Send notifications to Slack channels via incoming webhooks.",
		Category:         "notifications",
		Icon:             "#",
		Color:            "#4a154b",
		SupportsTriggers: true,
		ConfigFields: []ConfigFieldDef{
			{Key: "webhook_url", Label: "Webhook URL", Type: "url", Required: true, Placeholder: "https://hooks.slack.com/services/..."},
			{Key: "channel", Label: "Channel", Type: "text", Placeholder: "#general"},
		},
	},
	{
		ID:          "algolia",
		Name:        "Algolia",
		Description: "Add instant search to your collections with Algolia.",
		Category:    "search",
		Icon:        "A",
		Color:       "#003dff",
		ConfigFields: []ConfigFieldDef{
			{Key: "app_id", Label: "Application ID", Type: "text", Required: true},
			{Key: "api_key", Label: "Admin API Key", Type: "password", Required: true},
			{Key: "search_key", Label: "Search-Only API Key", Type: "password"},
		},
	},
	{
		ID:               "onesignal",
		Name:             "OneSignal",
		Description:      "Send push notifications to web and mobile via OneSignal.",
		Category:         "notifications",
		Icon:             "O",
		Color:            "#e54b4b",
		SupportsTriggers: true,
		ConfigFields: []ConfigFieldDef{
			{Key: "app_id", Label: "App ID", Type: "text", Required: true},
			{Key: "rest_api_key", Label: "REST API Key", Type: "password", Required: true},
		},
	},
}

// GetByID returns the integration definition for the given ID, or nil.
func GetByID(id string) *IntegrationDef {
	for i := range Catalog {
		if Catalog[i].ID == id {
			return &Catalog[i]
		}
	}
	return nil
}
