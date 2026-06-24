// Package scenarios embeds the built-in scenario profiles shipped under
// scenarios/ so callers can read them without depending on the process working
// directory.
package scenarios

import "embed"

// Files holds the built-in scenario profile YAML files, addressed by their base
// name (for example "medium.yaml").
//
//go:embed small.yaml medium.yaml large.yaml
var Files embed.FS
