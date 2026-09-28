package boot

import (
	"os"
	"strings"

	"kwu/internal/config"
	"kwu/internal/pythonruntime"
	"kwu/internal/tool"
)

func pythonCommandEnvProvider(cfg *config.Config, workspaceRoot string) tool.CommandEnvProvider {
	mode := "auto"
	interpreter := ""
	if cfg != nil {
		mode = cfg.PythonRuntimeMode()
		interpreter = cfg.PythonInterpreter()
	}
	if value := strings.TrimSpace(os.Getenv("KWU_PYTHON_MODE")); value != "" {
		mode = value
	}
	if value := strings.TrimSpace(os.Getenv("KWU_PYTHON_INTERPRETER")); value != "" {
		interpreter = value
	}
	return pythonruntime.NewResolver(pythonruntime.Options{
		Mode:          mode,
		WorkspaceRoot: workspaceRoot,
		Interpreter:   interpreter,
	})
}
