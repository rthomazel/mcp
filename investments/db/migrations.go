// Package db embeds the SQL migrations applied at server start.
package db

import "embed"

// Migrations holds the versioned up migrations under migrations/.
//
//go:embed migrations/*.sql
var Migrations embed.FS
