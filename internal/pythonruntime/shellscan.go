package pythonruntime

import (
	"runtime"
	"strings"
)

// HasShellCommand reports whether text invokes one of want outside heredoc data.
func HasShellCommand(text string, want map[string]struct{}) bool {
	_, strictDynamic := want["python"]
	powerShellScanner := shellCommandScanner{text: text, want: want, expectCommand: true, strictDynamic: strictDynamic, powerShell: true}
	powerShellMatches := powerShellScanner.commandMatches(true)
	if powerShellScanner.sawPowerShellAssignment {
		return len(powerShellMatches) > 0
	}
	scanner := shellCommandScanner{text: text, want: want, expectCommand: true, strictDynamic: strictDynamic}
	return len(scanner.commandMatches(true)) > 0
}

// ScanPythonShellBindings returns literal and unprovable dynamic Python calls.
func ScanPythonShellBindings(text string, want map[string]struct{}) []ShellCommandBinding {
	scanner := shellCommandScanner{text: text, want: want, expectCommand: true, strictDynamic: true}
	return scanner.commandMatches(false)
}

// ScanPowerShellPythonBindings applies PowerShell command-position semantics.
func ScanPowerShellPythonBindings(text string, want map[string]struct{}) []ShellCommandBinding {
	scanner := shellCommandScanner{
		text: text, want: want, expectCommand: true, strictDynamic: true, powerShell: true,
	}
	return scanner.commandMatches(false)
}

type shellCommandScanner struct {
	text                    string
	want                    map[string]struct{}
	expectCommand           bool
	strictDynamic           bool
	powerShell              bool
	invokeValue             bool
	skipNextWord            bool
	wrapper                 shellCommandWrapper
	candidateVia            string
	persistentMutation      string
	pendingMutation         string
	mutationCommand         shellMutationCommand
	commandSeen             bool
	sawPowerShellAssignment bool
	heredocs                []shellHeredoc
	matches                 []ShellCommandBinding
}

const (
	pathMutationReason   = "command mutates PATH/PATHEXT before execution"
	aliasMutationReason  = "command defines a shell alias or function that can override interpreter resolution"
	hashMutationReason   = "command changes the shell command hash table before execution"
	sourceMutationReason = "source command may change Python resolution before execution"
)

type shellMutationCommand struct {
	kind       string
	nextRole   string
	positional int
	hashStage  int
}

func (s *shellCommandScanner) commandMatches(stopAtFirst bool) []ShellCommandBinding {
	for i := 0; i < len(s.text); {
		matched, next := s.scanAt(i)
		if matched && stopAtFirst {
			break
		}
		i = next
	}
	s.finishCommand()
	return s.matches
}

func (s *shellCommandScanner) finishCommand() {
	if s.pendingMutation != "" && !s.commandSeen {
		s.persistentMutation = s.pendingMutation
	}
}

func (s *shellCommandScanner) scanAt(i int) (bool, int) {
	c := s.text[i]
	if isShellSpace(c) {
		return false, s.scanSpace(i, c)
	}
	if strings.ContainsRune(";&|(){}", rune(c)) {
		return false, s.scanBoundary(i, c)
	}
	if c == '<' || c == '>' {
		return false, s.consumeRedirect(i)
	}
	return s.scanWord(i)
}

func (s *shellCommandScanner) scanSpace(i int, c byte) int {
	if c != '\n' {
		return i + 1
	}
	s.resetCommand(true)
	if len(s.heredocs) == 0 {
		return i + 1
	}
	return s.skipHeredocBodies(i + 1)
}

func (s *shellCommandScanner) scanBoundary(i int, c byte) int {
	switch c {
	case ';', '|', '(':
		s.resetCommand(true)
		return shellBoundaryEnd(s.text, i, "||")
	case '&':
		s.resetCommand(true)
		end := shellBoundaryEnd(s.text, i, "&&")
		if end == i+1 {
			s.invokeValue = s.powerShell
		}
		return end
	case '{':
		s.scanOpenBrace(i)
	case ')', '}':
		s.resetCommand(false)
	}
	return i + 1
}

func shellBoundaryEnd(text string, start int, pair string) int {
	if strings.HasPrefix(text[start:], pair) {
		return start + len(pair)
	}
	return start + 1
}

func (s *shellCommandScanner) scanOpenBrace(i int) {
	if i == 0 || s.text[i-1] != '$' {
		s.resetCommand(true)
	}
}

func (s *shellCommandScanner) scanWord(i int) (bool, int) {
	word, next := readShellWord(s.text, i)
	if s.isFileDescriptor(word, next) {
		return false, next
	}
	if assignment, ok := s.powerShellAssignment(word, next); ok {
		return false, assignment
	}
	if s.isFunctionDeclaration(word, next) {
		s.persistentMutation = aliasMutationReason
		s.commandSeen = true
		s.expectCommand = false
		return false, next
	}
	return s.consumeWord(word), next
}

func (s *shellCommandScanner) isFileDescriptor(word string, next int) bool {
	return s.expectCommand && word != "" && strings.Trim(word, "0123456789") == "" && next < len(s.text) && (s.text[next] == '<' || s.text[next] == '>')
}

func (s *shellCommandScanner) powerShellAssignment(word string, next int) (int, bool) {
	if !s.powerShell || !s.expectCommand || s.wrapper.kind != "" || !strings.HasPrefix(word, "$") {
		return 0, false
	}
	if name, ok := powerShellInlineAssignmentName(word); ok {
		s.sawPowerShellAssignment = true
		s.recordPowerShellAssignment(name)
		s.expectCommand = false
		s.commandSeen = true
		return next, true
	}
	next = skipShellSpace(s.text, next, true)
	afterOperator, ok := powerShellAssignmentOperator(s.text, next)
	if !ok {
		return 0, false
	}
	s.sawPowerShellAssignment = true
	s.recordPowerShellAssignment(word)
	s.commandSeen = true
	return s.consumePowerShellAssignmentValue(skipShellSpace(s.text, afterOperator, false)), true
}

func (s *shellCommandScanner) recordPowerShellAssignment(name string) {
	if isPowerShellResolutionEnvironment(name) {
		s.persistentMutation = pathMutationReason
	}
}

func powerShellInlineAssignmentName(word string) (string, bool) {
	for _, operator := range []string{"??=", "+=", "-=", "*=", "/=", "%=", "="} {
		if index := strings.Index(word, operator); index > 0 {
			return word[:index], true
		}
	}
	return "", false
}

func powerShellAssignmentOperator(text string, start int) (int, bool) {
	for _, operator := range []string{"??=", "+=", "-=", "*=", "/=", "%=", "="} {
		if strings.HasPrefix(text[start:], operator) {
			return start + len(operator), true
		}
	}
	return start, false
}

func skipShellSpace(text string, next int, includeNewline bool) int {
	for next < len(text) && isShellSpace(text[next]) {
		if !includeNewline && text[next] == '\n' {
			break
		}
		next++
	}
	return next
}

func (s *shellCommandScanner) consumePowerShellAssignmentValue(start int) int {
	s.expectCommand = false
	if start >= len(s.text) || !isShellQuote(s.text[start]) {
		return start
	}
	quote := s.text[start]
	value, end := readQuotedShellWord(s.text, start+1, quote)
	if quote == '"' && shellWordMayContainCommand(value) && s.captureNested(value, "powershell-assignment") {
		return min(end+1, len(s.text))
	}
	return min(end+1, len(s.text))
}

func isShellQuote(value byte) bool {
	return value == '\'' || value == '"'
}

func (s *shellCommandScanner) consumeRedirect(i int) int {
	if isHeredocStart(s.text, i) {
		return s.consumeHeredoc(i)
	}
	s.skipNextWord = true
	return skipShellRedirect(s.text, i)
}

func (s *shellCommandScanner) resetCommand(expect bool) {
	s.finishCommand()
	s.expectCommand = expect
	s.invokeValue = false
	s.skipNextWord = false
	s.wrapper = shellCommandWrapper{}
	s.candidateVia = ""
	s.pendingMutation = ""
	s.mutationCommand = shellMutationCommand{}
	s.commandSeen = false
}

func (s *shellCommandScanner) consumeWord(word string) bool {
	if word == "" {
		return false
	}
	if s.skipNextWord {
		s.skipNextWord = false
		return false
	}
	if !s.expectCommand {
		return false
	}
	if s.mutationCommand.kind != "" {
		s.consumeMutationArgument(word)
		return false
	}
	if isShellAssignment(word) {
		s.consumeAssignment(word)
		return false
	}
	candidate, matched := s.resolveWrappedWord(word)
	if matched || !candidate {
		return matched
	}
	matched = s.consumeCommandWord(word)
	s.candidateVia = ""
	return matched
}

func (s *shellCommandScanner) consumeAssignment(word string) {
	name, _, _ := strings.Cut(word, "=")
	if isResolutionEnvironment(name) {
		s.pendingMutation = pathMutationReason
	}
}

func (s *shellCommandScanner) resolveWrappedWord(word string) (candidate, matched bool) {
	if s.wrapper.kind == "" {
		return true, false
	}
	wrapper := s.wrapper.kind
	if s.wrapper.prefix != "" {
		wrapper = s.wrapper.prefix + ">" + wrapper
	}
	candidate, nested := s.wrapper.commandCandidate(word)
	if nested && s.captureNested(word, wrapper) {
		return false, true
	}
	if candidate {
		if s.wrapper.mutation != "" {
			s.pendingMutation = s.wrapper.mutation
		}
		s.candidateVia = wrapper
		s.wrapper = shellCommandWrapper{}
	}
	return candidate, false
}

func (s *shellCommandScanner) captureNested(word, wrapper string) bool {
	nested := shellCommandScanner{
		text: word, want: s.want, expectCommand: true, strictDynamic: s.strictDynamic,
		powerShell: s.nestedPowerShell(wrapper),
	}
	matches := nested.commandMatches(true)
	if nested.persistentMutation != "" && nestedMutationPersists(wrapper) {
		s.persistentMutation = nested.persistentMutation
	}
	if len(matches) == 0 {
		return false
	}
	s.matches = append(s.matches, ShellCommandBinding{Command: word, Wrapper: wrapper, Unprovable: s.withActiveMutation("nested command string has shell-specific resolution semantics")})
	s.expectCommand = false
	return true
}

func nestedMutationPersists(wrapper string) bool {
	parts := strings.Split(wrapper, ">")
	last := parts[len(parts)-1]
	return last == "eval" || last == "powershell-eval"
}

func (s *shellCommandScanner) consumeCommandWord(word string) bool {
	base := normalizedShellCommandBase(word)
	if _, ok := s.want[base]; ok {
		s.commandSeen = true
		s.matches = append(s.matches, ShellCommandBinding{Command: word, Wrapper: s.candidateVia, Unprovable: s.activeMutation()})
		s.expectCommand = false
		return true
	}
	if kind := mutationCommandKind(base, s.powerShell); kind != "" {
		s.beginMutationCommand(kind)
		return false
	}
	if kind := shellWrapperKind(base); kind != "" {
		if kind == "powershell-eval" && s.powerShell {
			s.persistentMutation = powerShellEvalMutationReason
		}
		s.commandSeen = true
		s.wrapper = shellCommandWrapper{kind: kind, prefix: s.candidateVia}
		return false
	}
	if base == "source" || base == "." {
		s.commandSeen = true
		s.persistentMutation = sourceMutationReason
		s.expectCommand = false
		return false
	}
	if s.dynamicCommandRequiresProof(word) {
		s.commandSeen = true
		s.matches = append(s.matches, ShellCommandBinding{Command: word, Wrapper: s.candidateVia, Unprovable: s.withActiveMutation("dynamic command word may resolve to an unverified Python interpreter")})
		s.expectCommand = false
		return true
	}
	s.commandSeen = true
	s.expectCommand = false
	return false
}

func (s *shellCommandScanner) beginMutationCommand(kind string) {
	if mutationCommandPromotesAssignment(kind) && s.pendingMutation != "" {
		s.persistentMutation = s.pendingMutation
	}
	s.commandSeen = true
	s.mutationCommand = shellMutationCommand{kind: kind}
}

func mutationCommandPromotesAssignment(kind string) bool {
	return kind == "export" || kind == "declaration"
}

func (s *shellCommandScanner) activeMutation() string {
	if s.pendingMutation != "" {
		return s.pendingMutation
	}
	return s.persistentMutation
}

func (s *shellCommandScanner) withActiveMutation(reason string) string {
	if mutation := s.activeMutation(); mutation != "" {
		return reason + "; " + mutation
	}
	return reason
}

func (s *shellCommandScanner) isFunctionDeclaration(word string, next int) bool {
	if s.powerShell || !s.expectCommand || s.wrapper.kind != "" || !s.isPythonMutationTarget(word) {
		return false
	}
	next = skipShellSpace(s.text, next, false)
	if next >= len(s.text) || s.text[next] != '(' {
		return false
	}
	next = skipShellSpace(s.text, next+1, false)
	return next < len(s.text) && s.text[next] == ')'
}

func (s *shellCommandScanner) dynamicCommandRequiresProof(word string) bool {
	if !s.strictDynamic || !isDynamicShellCommandWord(word) {
		return false
	}
	if !s.powerShell || strings.Contains(word, "`") {
		return true
	}
	return s.invokeValue
}

func (s *shellCommandScanner) nestedPowerShell(wrapper string) bool {
	lower := strings.ToLower(wrapper)
	if strings.HasSuffix(lower, "powershell") || strings.HasSuffix(lower, "powershell-assignment") {
		return true
	}
	if strings.HasSuffix(lower, "shell") || strings.HasSuffix(lower, "cmd") {
		return false
	}
	return s.powerShell
}

type shellCommandWrapper struct {
	kind         string
	prefix       string
	skipNext     bool
	commandReady bool
	seenDuration bool
	unsetNext    bool
	mutation     string
}

func (w *shellCommandWrapper) commandCandidate(word string) (candidate, nested bool) {
	if w.skipNext {
		w.skipNext = false
		return false, false
	}
	if fn := shellWrapperCandidateFuncs[w.kind]; fn != nil {
		return fn(w, word)
	}
	return true, false
}

var shellWrapperCandidateFuncs = map[string]func(*shellCommandWrapper, string) (bool, bool){
	"transparent":     transparentWrapperCandidate,
	"env":             envWrapperCandidate,
	"sudo":            sudoWrapperCandidate,
	"timeout":         timeoutWrapperCandidate,
	"cmd":             cmdWrapperCandidate,
	"shell":           shellWrapperCandidate,
	"powershell":      powershellWrapperCandidate,
	"start-process":   startProcessWrapperCandidate,
	"eval":            evalWrapperCandidate,
	"powershell-eval": evalWrapperCandidate,
	"builtin":         transparentWrapperCandidate,
}

var (
	cmdCommandOptions        = map[string]struct{}{`/c`: {}, `/k`: {}}
	shellCommandOptions      = map[string]struct{}{`-c`: {}, `--command`: {}}
	powershellCommandOptions = map[string]struct{}{`-c`: {}, `-command`: {}}
)

func transparentWrapperCandidate(_ *shellCommandWrapper, word string) (bool, bool) {
	return !strings.HasPrefix(word, "-"), false
}

func envWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	if consumeEnvUnsetTarget(w, word) {
		return false, false
	}
	if w.commandReady {
		return true, shellWordMayContainCommand(word)
	}
	if consumeEnvAssignment(w, word) {
		return false, false
	}
	return consumeEnvOption(w, word)
}

func consumeEnvUnsetTarget(w *shellCommandWrapper, word string) bool {
	if !w.unsetNext {
		return false
	}
	w.unsetNext = false
	if isResolutionEnvironment(word) {
		w.mutation = pathMutationReason
	}
	return true
}

func consumeEnvAssignment(w *shellCommandWrapper, word string) bool {
	if !isShellAssignment(word) {
		return false
	}
	name, _, _ := strings.Cut(word, "=")
	if isResolutionEnvironment(name) {
		w.mutation = pathMutationReason
	}
	return true
}

func consumeEnvOption(w *shellCommandWrapper, word string) (bool, bool) {
	lower := strings.ToLower(word)
	if lower == "-u" || lower == "--unset" {
		w.unsetNext = true
		return false, false
	}
	if strings.HasPrefix(lower, "--unset=") {
		if isResolutionEnvironment(strings.TrimPrefix(lower, "--unset=")) {
			w.mutation = pathMutationReason
		}
		return false, false
	}
	if word == "-S" || lower == "--split-string" {
		w.commandReady = true
		return false, false
	}
	if shellOptionNeedsValue(lower, "-c", "--chdir") {
		w.skipNext = true
		return false, false
	}
	return !strings.HasPrefix(word, "-"), false
}

func sudoWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	lower := strings.ToLower(word)
	if shellOptionNeedsValue(lower, "-c", "--close-from", "-d", "--chdir", "-g", "--group", "-h", "--host", "-p", "--prompt", "-r", "--role", "-t", "--type", "-u", "--user") {
		w.skipNext = true
		return false, false
	}
	return !strings.HasPrefix(word, "-"), false
}

func timeoutWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	if shellOptionNeedsValue(strings.ToLower(word), "-k", "--kill-after", "-s", "--signal") {
		w.skipNext = true
		return false, false
	}
	if strings.HasPrefix(word, "-") {
		return false, false
	}
	if !w.seenDuration {
		w.seenDuration = true
		return false, false
	}
	return true, false
}

func cmdWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	return commandStringWrapperCandidate(w, strings.ToLower(word), cmdCommandOptions)
}

func shellWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	return commandStringWrapperCandidate(w, word, shellCommandOptions)
}

func powershellWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	return commandStringWrapperCandidate(w, strings.ToLower(word), powershellCommandOptions)
}

func commandStringWrapperCandidate(w *shellCommandWrapper, word string, options map[string]struct{}) (bool, bool) {
	if w.commandReady {
		return true, shellWordMayContainCommand(word)
	}
	if _, ok := options[word]; ok {
		w.commandReady = true
	}
	return false, false
}

func startProcessWrapperCandidate(w *shellCommandWrapper, word string) (bool, bool) {
	if w.commandReady {
		return true, false
	}
	lower := strings.ToLower(word)
	if lower == "-filepath" {
		w.commandReady = true
		return false, false
	}
	if shellOptionNeedsValue(lower, "-argumentlist", "-credential", "-redirectstandarderror", "-redirectstandardinput", "-redirectstandardoutput", "-verb", "-windowstyle", "-workingdirectory") {
		w.skipNext = true
		return false, false
	}
	return !strings.HasPrefix(word, "-"), false
}

func evalWrapperCandidate(_ *shellCommandWrapper, word string) (bool, bool) {
	if word == "--" {
		return false, false
	}
	return false, true
}

func isDynamicShellCommandWord(word string) bool {
	if strings.Contains(word, "`") {
		return true
	}
	if !strings.HasPrefix(word, "$") {
		return false
	}
	switch strings.ToLower(word) {
	case "$true", "$false", "$null", "$?":
		return false
	default:
		return true
	}
}

func shellWrapperKind(base string) string {
	return shellWrapperKinds[strings.ToLower(base)]
}

var shellWrapperKinds = map[string]string{
	"!": "transparent", "call": "transparent", "command": "transparent", "do": "transparent", "elif": "transparent",
	"else": "transparent", "exec": "transparent", "if": "transparent", "nohup": "transparent", "setsid": "transparent",
	"then": "transparent", "time": "transparent", "until": "transparent", "while": "transparent",
	"env": "env", "sudo": "sudo", "timeout": "timeout", "gtimeout": "timeout",
	"cmd": "cmd", "cmd.exe": "cmd", "sh": "shell", "bash": "shell", "dash": "shell", "ksh": "shell", "zsh": "shell",
	"pwsh": "powershell", "pwsh.exe": "powershell", "powershell": "powershell", "powershell.exe": "powershell",
	"start-process": "start-process", "eval": "eval", "invoke-expression": "powershell-eval", "iex": "powershell-eval",
	"builtin": "builtin",
}

func shellOptionNeedsValue(word string, names ...string) bool {
	for _, name := range names {
		if word == name {
			return true
		}
		if strings.HasPrefix(name, "--") && strings.HasPrefix(word, name+"=") {
			return false
		}
	}
	return false
}

func shellWordMayContainCommand(word string) bool {
	return strings.ContainsAny(word, ";|&(){}\n\r \t")
}

func normalizedShellCommandBase(word string) string {
	base := shellWordBase(word)
	if runtime.GOOS == "windows" {
		return strings.ToLower(base)
	}
	return base
}

func readShellWord(text string, start int) (string, int) {
	var b strings.Builder
	for i := start; i < len(text); i++ {
		c := text[i]
		if isShellSpace(c) || strings.ContainsRune(";|&(){}<>", rune(c)) {
			return b.String(), i
		}
		switch c {
		case '\\':
			if i+1 < len(text) {
				b.WriteByte(c)
			}
		case '\'', '"':
			fragment, end := readQuotedShellWord(text, i+1, c)
			b.WriteString(fragment)
			i = end
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), len(text)
}

func readQuotedShellWord(text string, start int, quote byte) (string, int) {
	end := strings.IndexByte(text[start:], quote)
	if end < 0 {
		return text[start:], len(text)
	}
	end += start
	return text[start:end], end
}

func skipShellRedirect(text string, i int) int {
	for i < len(text) && (text[i] == '<' || text[i] == '>' || text[i] == '&') {
		i++
	}
	return i
}

func isShellSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func isShellAssignment(word string) bool {
	name, _, ok := strings.Cut(word, "=")
	if !ok || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isShellNameCharacter(name[i], i == 0) {
			return false
		}
	}
	return true
}

func isShellNameCharacter(c byte, first bool) bool {
	letter := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
	if first {
		return c == '_' || letter
	}
	return c == '_' || letter || (c >= '0' && c <= '9')
}

func shellWordBase(word string) string {
	if i := strings.LastIndexAny(word, "/\\"); i >= 0 {
		return word[i+1:]
	}
	return word
}
