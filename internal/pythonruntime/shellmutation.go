package pythonruntime

import "strings"

const powerShellEvalMutationReason = "Invoke-Expression may change Python resolution before execution"

func (s *shellCommandScanner) consumeMutationArgument(word string) {
	command := &s.mutationCommand
	switch command.kind {
	case "export":
		s.consumeExportArgument(word)
	case "unset":
		s.consumeUnsetArgument(word)
	case "alias":
		s.consumeAliasArgument(word)
	case "function":
		s.consumeFunctionArgument(command, word)
	case "hash":
		s.consumeHashArgument(command, word)
	case "set-alias":
		s.consumeSetAliasArgument(command, word)
	case "set-item":
		s.consumeSetItemArgument(command, word)
	case "declaration":
		s.consumeDeclarationArgument(word)
	}
}

func (s *shellCommandScanner) consumeDeclarationArgument(word string) {
	name, _, assigned := strings.Cut(word, "=")
	if assigned && isResolutionEnvironment(name) {
		s.persistentMutation = pathMutationReason
	}
}

func (s *shellCommandScanner) consumeExportArgument(word string) {
	name, _, assigned := strings.Cut(word, "=")
	if assigned && isResolutionEnvironment(name) {
		s.persistentMutation = pathMutationReason
	}
}

func (s *shellCommandScanner) consumeUnsetArgument(word string) {
	if !strings.HasPrefix(word, "-") && isResolutionEnvironment(word) {
		s.persistentMutation = pathMutationReason
	}
}

func (s *shellCommandScanner) consumeAliasArgument(word string) {
	name, _, _ := strings.Cut(word, "=")
	if s.isPythonMutationTarget(name) {
		s.persistentMutation = aliasMutationReason
	}
}

func (s *shellCommandScanner) consumeFunctionArgument(command *shellMutationCommand, word string) {
	if command.positional != 0 || strings.HasPrefix(word, "-") {
		return
	}
	command.positional++
	if s.isPythonMutationTarget(word) {
		s.persistentMutation = aliasMutationReason
	}
}

func (s *shellCommandScanner) consumeHashArgument(command *shellMutationCommand, word string) {
	if strings.EqualFold(word, "-p") {
		command.hashStage = 1
		return
	}
	if command.hashStage == 1 {
		command.hashStage = 2
		return
	}
	if command.hashStage != 2 {
		return
	}
	command.hashStage = 0
	if s.isPythonMutationTarget(word) {
		s.persistentMutation = hashMutationReason
	}
}

func (s *shellCommandScanner) consumeSetAliasArgument(command *shellMutationCommand, word string) {
	if command.nextRole != "" {
		role := command.nextRole
		command.nextRole = ""
		if role == "name" && s.isPythonMutationTarget(word) {
			s.persistentMutation = aliasMutationReason
		}
		return
	}
	role, option := setAliasOptionRole(word)
	if option {
		command.nextRole = role
		return
	}
	if command.positional == 0 {
		command.positional++
		if s.isPythonMutationTarget(word) {
			s.persistentMutation = aliasMutationReason
		}
	}
}

func setAliasOptionRole(word string) (string, bool) {
	switch strings.ToLower(word) {
	case "-name":
		return "name", true
	case "-value", "-description", "-option", "-scope":
		return "ignored", true
	default:
		return "", strings.HasPrefix(word, "-")
	}
}

func (s *shellCommandScanner) consumeSetItemArgument(command *shellMutationCommand, word string) {
	if command.nextRole != "" {
		role := command.nextRole
		command.nextRole = ""
		if role == "path" {
			s.recordPowerShellItemMutation(word)
		}
		return
	}
	role, option := setItemOptionRole(word)
	if option {
		command.nextRole = role
		return
	}
	if command.positional == 0 {
		command.positional++
		s.recordPowerShellItemMutation(word)
	}
}

func setItemOptionRole(word string) (string, bool) {
	switch strings.ToLower(word) {
	case "-path", "-literalpath":
		return "path", true
	case "-value", "-filter", "-include", "-exclude", "-credential":
		return "ignored", true
	default:
		return "", strings.HasPrefix(word, "-")
	}
}

func (s *shellCommandScanner) recordPowerShellItemMutation(word string) {
	provider, target, ok := powerShellProviderTarget(word)
	if !ok {
		return
	}
	if provider == "env" && isResolutionEnvironment(target) {
		s.persistentMutation = pathMutationReason
		return
	}
	if provider == "alias" || provider == "function" {
		if s.isPythonMutationTarget(target) {
			s.persistentMutation = aliasMutationReason
		}
	}
}

func powerShellProviderTarget(word string) (provider, target string, ok bool) {
	normalized := strings.ReplaceAll(strings.TrimSpace(word), "\\", "/")
	provider, target, ok = strings.Cut(normalized, ":")
	if !ok {
		return "", "", false
	}
	provider = strings.ToLower(provider)
	target = strings.TrimLeft(target, "/")
	return provider, target, target != ""
}

func (s *shellCommandScanner) isPythonMutationTarget(word string) bool {
	word = strings.TrimSpace(strings.TrimSuffix(word, "()"))
	if colon := strings.LastIndexByte(word, ':'); colon >= 0 {
		word = word[colon+1:]
	}
	_, ok := s.want[strings.ToLower(word)]
	return ok
}

func mutationCommandKind(base string, powerShell bool) string {
	base = strings.ToLower(base)
	if powerShell {
		return powerShellMutationCommands[base]
	}
	return bashMutationCommands[base]
}

var bashMutationCommands = map[string]string{
	"alias": "alias", "export": "export", "function": "function", "hash": "hash", "unset": "unset",
	"local": "declaration", "declare": "declaration", "typeset": "declaration", "readonly": "declaration",
}

var powerShellMutationCommands = map[string]string{
	"function": "function", "nal": "set-alias", "new-alias": "set-alias", "sal": "set-alias", "set-alias": "set-alias",
	"set-item": "set-item", "si": "set-item", "new-item": "set-item", "ni": "set-item",
	"clear-item": "set-item", "cli": "set-item", "remove-item": "set-item", "ri": "set-item", "rm": "set-item",
	"rmdir": "set-item", "del": "set-item", "erase": "set-item", "rd": "set-item",
}

func isPowerShellResolutionEnvironment(word string) bool {
	provider, target, ok := powerShellProviderTarget(strings.TrimPrefix(word, "$"))
	return ok && provider == "env" && isResolutionEnvironment(target)
}

func isResolutionEnvironment(name string) bool {
	return strings.EqualFold(name, "PATH") || strings.EqualFold(name, "PATHEXT")
}
