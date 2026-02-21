package httpapi

import (
	"strings"

	"github.com/ketsuna-org/sovrabase/internal/config"
)

func sanitizeConfig(cfg config.Config, encryptionConfigured, jwtConfigured bool) sanitizedConfig {
	return sanitizedConfig{
		Server: sanitizedServer{
			Host: cfg.Server.Host,
			Port: cfg.Server.Port,
		},
		Metadata: sanitizedMetadata{
			Driver:             cfg.Metadata.Driver,
			SQLiteConfigured:   strings.TrimSpace(cfg.Metadata.SQLite.Path) != "",
			PostgresConfigured: strings.TrimSpace(cfg.Metadata.Postgres.DSN) != "",
		},
		Core: sanitizedCore{
			CacheTTL:                cfg.Core.CacheTTL,
			SweepInterval:           cfg.Core.Sweep,
			EncryptionKeyConfigured: encryptionConfigured,
		},
		Auth: sanitizedAuth{
			JWTSigningKeyConfigured: jwtConfigured,
		},
		Provisioning: sanitizedProvisioning{
			DefaultProvider: cfg.Provisioning.DefaultProvider,
			Docker: sanitizedDocker{
				Enabled:       cfg.Provisioning.Docker.Enabled,
				Mode:          cfg.Provisioning.Docker.Mode,
				PostgresImage: cfg.Provisioning.Docker.PostgresImage,
				MongoImage:    cfg.Provisioning.Docker.MongoImage,
				Endpoint:      "[redacted]",
				HostAddress:   "[redacted]",
				NetworkName:   "[redacted]",
			},
		},
	}
}
