package main

import _ "embed"

// Embed the executable task and grader contracts so the manifest changes when
// either implementation changes. Reports still expose only hashes.

//go:embed desktop_tasks.go
var desktopTaskSource string

//go:embed desktop_graders.go
var desktopGraderSource string
