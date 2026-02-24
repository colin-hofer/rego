package app

import (
	"database/sql"

	"rego/internal/logx"
	"rego/internal/modules/metadata"
	"rego/internal/modules/system"
	"rego/internal/server"
)

func loadServerModules(baseLogger *logx.Logger, database *sql.DB) []server.Module {
	// Register backend feature modules here.
	return []server.Module{
		system.New(system.Options{
			Logger:   baseLogger.WithComponent("system"),
			Database: database,
		}),
		metadata.New(metadata.Options{
			Logger: baseLogger.WithComponent("metadata"),
			DB:     database,
		}),
	}
}
