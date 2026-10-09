// Package migrations embeds the SQL migration files into the binary.
package migrations

import "embed"

// FS holds every migration file in this directory.
//
//go:embed *.sql
var FS embed.FS
