// Command ctfcode is a config- and plugin-driven coding agent CLI.
package main

import (
	"os"

	"ctfcode/internal/cli"

	// Blank imports wire compile-time built-ins into their registries.
	_ "ctfcode/internal/provider/anthropic"
	_ "ctfcode/internal/provider/openai"
	_ "ctfcode/internal/tool/builtin"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
