package migrations

import "embed"

// FS contains the ordered SQL migrations.
//
//go:embed *.sql
var FS embed.FS
