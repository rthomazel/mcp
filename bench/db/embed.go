// Package db embeds migration files for use with golang-migrate.
package db

import "embed"

// Migrations holds all .up.sql migration files.
//
//go:embed migrations/*.up.sql
var Migrations embed.FS
