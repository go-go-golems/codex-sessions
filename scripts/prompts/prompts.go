package prompts

import "embed"

// FS embeds the built-in prompt presets and their version state files.
//
// These are used by the Go port of the reflection CLI.
//
//go:embed *.md *_version.json
var FS embed.FS
