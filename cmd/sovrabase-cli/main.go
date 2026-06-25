package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ketsuna-org/sovrabase/client"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "login":
		cmdLogin()
	case "project":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sovrabase project <list|create|delete|config>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			cmdProjectList()
		case "create":
			cmdProjectCreate()
		case "config":
			cmdProjectConfig()
		default:
			fmt.Printf("Unknown project subcommand: %s\n", os.Args[2])
		}
	case "collections":
		cmdCollections()
	case "config":
		cmdConfig()
	case "cron":
		cmdCron()
	case "webhooks":
		cmdWebhooks()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`Sovrabase CLI - Sovereign European BaaS

Usage: sovrabase <command> [subcommand] [options]

Commands:
  login                    Authenticate and save API key
  project list             List all projects
  project create <name>    Create a new project
  project config           Show project remote config
  collections <name>       List collections in a project
  config list|set|get|del  Manage remote config entries
  cron list|create|delete  Manage scheduled cron jobs
  webhooks list|create     Manage webhooks

Environment:
  SOVRABASE_URL            Server URL (default: http://localhost:6070)
  SOVRABASE_API_KEY        Project API key`)
}

func getClient() (*client.Client, string) {
	baseURL := os.Getenv("SOVRABASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:6070"
	}
	projectKey := os.Getenv("SOVRABASE_API_KEY")
	apiKey := os.Getenv("SOVRABASE_API_KEY")
	_ = apiKey
	return client.New(baseURL, projectKey), projectKey
}

func must(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdLogin() {
	email := prompt("Email")
	password := promptPassword("Password")
	baseURL := os.Getenv("SOVRABASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:6070"
	}

	// Simple HTTP login.
	resp, err := httpPost(fmt.Sprintf("%s/admin/login", baseURL), map[string]string{
		"email":    email,
		"password": password,
	})
	must(err)
	var result struct {
		Token string `json:"token"`
	}
	json.Unmarshal(resp, &result)
	if result.Token != "" {
		fmt.Println("Login successful. Token:")
		fmt.Println(result.Token)
		fmt.Println("\nExport it: export SOVRABASE_API_KEY=" + result.Token)
	}
}

func cmdProjectList() {
	// Use admin token to list projects.
	baseURL := os.Getenv("SOVRABASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:6070"
	}
	token := os.Getenv("SOVRABASE_API_KEY")
	if token == "" {
		fmt.Println("Please login first: sovrabase login")
		return
	}
	data, err := httpGet(fmt.Sprintf("%s/admin/projects", baseURL), token)
	must(err)
	var result struct {
		Projects []map[string]interface{} `json:"projects"`
	}
	json.Unmarshal(data, &result)
	fmt.Printf("%-40s %-10s %s\n", "ID", "Name", "Created")
	for _, p := range result.Projects {
		fmt.Printf("%-40s %-10s %v\n", p["id"], p["name"], p["created_at"])
	}
}

func cmdProjectCreate() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: sovrabase project create <name>")
		return
	}
	name := os.Args[3]
	baseURL := os.Getenv("SOVRABASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:6070"
	}
	token := os.Getenv("SOVRABASE_API_KEY")
	if token == "" {
		fmt.Println("Please login first: sovrabase login")
		return
	}
	body, _ := json.Marshal(map[string]string{"name": name})
	resp, err := httpPostAuth(fmt.Sprintf("%s/admin/projects", baseURL), token, body)
	must(err)
	var proj map[string]interface{}
	json.Unmarshal(resp, &proj)
	fmt.Printf("Project created!\nID: %s\nName: %s\nAPI Key: %s\n",
		proj["id"], proj["name"], proj["jwt_secret"])
}

func cmdProjectConfig() {
	baseURL := os.Getenv("SOVRABASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:6070"
	}
	projectKey := os.Getenv("SOVRABASE_API_KEY")
	c := client.New(baseURL, projectKey)
	entries, err := c.ConfigGetPublic()
	must(err)
	for k, v := range entries {
		fmt.Printf("%s = %v\n", k, v)
	}
}

func cmdCollections() {
	fmt.Println("Use sovrabase project config for now — collections require full auth.")
	_ = os.Args // placeholder
}

func cmdConfig() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: sovrabase config <list|set|get|del> [key] [value]")
		fmt.Println("Set SOVRABASE_URL, SOVRABASE_API_KEY and authenticate first.")
		return
	}
	baseURL := os.Getenv("SOVRABASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:6070"
	}
	projectKey := os.Getenv("SOVRABASE_API_KEY")
	c := client.New(baseURL, projectKey)
	sub := os.Args[2]
	switch sub {
	case "list":
		entries, err := c.ConfigGetAll()
		must(err)
		for _, e := range entries {
			fmt.Printf("%-30s %-10s %v\n", e.Key, e.Type, e.Value)
		}
	case "get":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sovrabase config get <key>")
			return
		}
		entry, err := c.ConfigGet(os.Args[3])
		must(err)
		fmt.Printf("Key: %s\nType: %s\nValue: %v\n", entry.Key, entry.Type, entry.Value)
	case "set":
		if len(os.Args) < 5 {
			fmt.Println("Usage: sovrabase config set <key> <value>")
			return
		}
		_, err := c.ConfigSet(os.Args[3], os.Args[4], "", "", false)
		must(err)
		fmt.Println("Config set successfully")
	case "del":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sovrabase config del <key>")
			return
		}
		must(c.ConfigDelete(os.Args[3]))
		fmt.Println("Config deleted")
	}
}

func cmdCron()    { fmt.Println("cron: use the admin dashboard for cron management") }
func cmdWebhooks() { fmt.Println("webhooks: use the admin dashboard for webhook management") }

func prompt(label string) string {
	fmt.Printf("%s: ", label)
	var s string
	fmt.Scanln(&s)
	return strings.TrimSpace(s)
}

func promptPassword(label string) string {
	fmt.Printf("%s: ", label)
	var s string
	fmt.Scanln(&s)
	return strings.TrimSpace(s)
}
