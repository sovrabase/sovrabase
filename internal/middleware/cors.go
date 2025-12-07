package middleware

import (
	"log"
	"net/http"
	"strings"
)

// CORSConfig holds the configuration for CORS middleware
type CORSConfig struct {
	Domain         string   // Domain principal de l'API (si configuré)
	AllowedOrigins []string // Origins autorisées pour CORS
}

// CORSMiddleware creates a middleware that validates the Host header
// and sets appropriate CORS headers based on the allowed origins
func CORSMiddleware(config *CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Ne bloque pas la documentation Swagger pour faciliter le debug.
			if strings.HasPrefix(r.URL.Path, "/docs") {
				next.ServeHTTP(w, r)
				return
			}

			// Récupérer le Host header
			host := r.Host

			// Récupérer l'Origin header (pour les requêtes CORS)
			origin := r.Header.Get("Origin")

			// Extraire le hostname du Host (sans le port)
			hostWithoutPort := strings.Split(host, ":")[0]

			// Vérification 1: Si un Domain principal est configuré, le Host doit correspondre
			if config.Domain != "" {
				// Autoriser localhost en développement même si un Domain est configuré
				isLocalhost := hostWithoutPort == "localhost" || hostWithoutPort == "127.0.0.1" || strings.HasPrefix(host, "[::")

				if !isLocalhost && hostWithoutPort != config.Domain && host != config.Domain {
					log.Printf("❌ Blocked request: host '%s' does not match configured domain '%s'", host, config.Domain)
					http.Error(w, "Forbidden: Invalid domain", http.StatusForbidden)
					return
				}

				if isLocalhost {
					log.Printf("⚠️  Allowing localhost/loopback address despite domain restriction: %s", host)
				}
			}

			// Vérification 2: CORS - vérifier les origins autorisées
			// Si aucune restriction CORS n'est configurée, on autorise tout et on reflète l'origine pour permettre les cookies.
			if len(config.AllowedOrigins) == 0 {
				if origin != "" {
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Max-Age", "3600")

					if r.Method == http.MethodOptions {
						w.WriteHeader(http.StatusOK)
						return
					}
				}

				log.Printf("✅ Allowed request from: %s (origin: %s)", host, origin)
				next.ServeHTTP(w, r)
				return
			}

			// Vérifier les origins CORS (quand il y a un header Origin)
			allowed := false
			matchedOrigin := ""

			if origin != "" {
				// Extraire le hostname de l'Origin (peut être avec protocole http:// ou https://)
				originHost := strings.TrimPrefix(origin, "http://")
				originHost = strings.TrimPrefix(originHost, "https://")
				originHost = strings.Split(originHost, ":")[0]

				for _, allowedOrigin := range config.AllowedOrigins {
					if originHost == allowedOrigin {
						allowed = true
						matchedOrigin = origin
						break
					}
				}

				if !allowed {
					log.Printf("❌ Blocked CORS request from unauthorized origin: %s (host: %s)", origin, host)
					http.Error(w, "Forbidden: Invalid origin", http.StatusForbidden)
					return
				}

				// Définir les headers CORS appropriés
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Origin", matchedOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "3600")

				// Gérer les requêtes preflight OPTIONS
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusOK)
					return
				}
			} else {
				// Pas de header Origin - ce n'est pas une requête CORS
				// Si un Domain est configuré, la vérification a déjà été faite plus haut
				// Si pas de Domain, on vérifie que le Host correspond à un des AllowedOrigins
				if config.Domain == "" {
					for _, allowedOrigin := range config.AllowedOrigins {
						if hostWithoutPort == allowedOrigin || host == allowedOrigin {
							allowed = true
							break
						}
					}

					// Si localhost est utilisé en développement, on peut l'autoriser
					if !allowed && (hostWithoutPort == "localhost" || hostWithoutPort == "127.0.0.1" || strings.HasPrefix(host, "[::")) {
						log.Printf("⚠️  Allowing localhost/loopback address: %s", host)
						allowed = true
					}

					if !allowed {
						log.Printf("❌ Blocked request from unauthorized host: %s", host)
						http.Error(w, "Forbidden: Invalid host", http.StatusForbidden)
						return
					}
				}
			}

			log.Printf("✅ Allowed request from: %s (origin: %s)", host, origin)
			next.ServeHTTP(w, r)
		})
	}
}
