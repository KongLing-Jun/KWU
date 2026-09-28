// Command kwu is a config- and plugin-driven coding agent CLI.
package main

import (
	"os"

	"kwu/internal/cli"

	// Blank imports wire compile-time built-ins into their registries.
	_ "kwu/internal/provider/anthropic"
	_ "kwu/internal/provider/openai"
	_ "kwu/internal/tool/builtin"
)

// version is injected at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Run(os.Args[1:], version))
}
