// Package inittempl holds the embedded meta-templates used by goplt init.
package inittempl

import "embed"

//go:embed all:templates
var FS embed.FS
