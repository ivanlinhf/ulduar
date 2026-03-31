package migrations

import "embed"

// Files contains all SQL migration files bundled into the backend binaries.
//
//go:embed *.sql
var Files embed.FS
