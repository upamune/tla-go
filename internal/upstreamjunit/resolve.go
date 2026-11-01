package upstreamjunit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Resolution separates a recovered invocation from unsupported harness and assertion behavior.
// A recovered invocation is not evidence that the JUnit test passes.
type Resolution struct {
	Case                                Case
	Path                                string
	Coverage, Dump, Deadlock, Translate bool
	Checkpoint                          string
	Reasons                             []string
}

var (
	constructor  = regexp.MustCompile(`(?s)\b(?:public|protected)\s+([\w$]+)\s*\(([^()]*)\)\s*\{\s*(?:super|this)\s*\(`)
	returnMethod = regexp.MustCompile(`(?s)\b(?:boolean|int)\s+(doCoverage|doDump|checkDeadLock|getNumberOfThreads|doCheckpoint)\s*\([^)]*\)\s*\{\s*return\s+([^;]+);\s*\}`)
	methodName   = regexp.MustCompile(`\b(?:boolean|int)\s+(doCoverage|doDump|checkDeadLock|getNumberOfThreads|doCheckpoint)\s*\(`)
	ecConst      = regexp.MustCompile(`(?m)\bint\s+(\w+)\s*=\s*(\d+)\s*;`)
	recorded     = regexp.MustCompile(`recorder\.(recordedWithStringValues|recordedWithStringValue|recorded)\s*\(\s*EC\.(\w+)`)
	pathJoin     = regexp.MustCompile(`^"([^"\\]*)"\s*\+\s*File\.separator\s*\+\s*"([^"\\]*)"$`)
)

type javaQuote struct{ quoted, escape bool }

func (q *javaQuote) consume(ch rune) bool {
	if !q.quoted {
		return false
	}
	if q.escape {
		q.escape = false
	} else if ch == '\\' {
		q.escape = true
	} else if ch == '"' {
		q.quoted = false
	}
	return true
}

// splitJava splits comma-separated expressions while respecting strings and delimiters.
func splitJava(s string) []string {
	var out []string
	start, depth := 0, 0
	var quote javaQuote
	for i, r := range s {
		if quote.consume(r) {
			continue
		}
		if r == '"' {
			quote.quoted = true
			continue
		}
		if r == ',' && depth == 0 {
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
			continue
		}
		depth += delimiterDepth(r)
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, strings.TrimSpace(s[start:]))
	}
	return out
}

func delimiterDepth(r rune) int {
	switch r {
	case '(', '{', '[':
		return 1
	case ')', '}', ']':
		return -1
	}
	return 0
}

func callEnd(s string, start int) int {
	depth := 1
	var quote javaQuote
	for i := start; i < len(s); i++ {
		ch := s[i]
		if quote.consume(rune(ch)) {
			continue
		}
		switch ch {
		case '"':
			quote.quoted = true
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func javaValue(s string, vars map[string]string) string {
	s = strings.TrimSpace(s)
	if v, ok := vars[s]; ok {
		return v
	}
	if s == "TLAConstants.Files.MODEL_CHECK_FILE_BASENAME" {
		return `"MC"`
	}
	return s
}

// Resolve handles literal constructor delegation through intermediate classes.
func Resolve(testRoot string, classes []Class, c Class) Resolution {
	r := Resolution{Case: Case{Name: c.Name, Workers: "1"}, Coverage: true, Dump: true, Checkpoint: "0", Translate: c.Kind == "PCal"}
	if c.Kind != "ModelChecker" && c.Kind != "PCal" {
		r.Reasons = append(r.Reasons, "unsupported harness: "+c.Kind)
		return r
	}
	resolver := newResolver(testRoot, classes, &r)
	args := resolver.resolveChain(c)
	r.resolveArguments(args)
	r.recordOmissions()
	return r
}

type resolver struct {
	root         string
	result       *Resolution
	byName       map[string]Class
	bySimple     map[string][]Class
	seen, chosen map[string]bool
}

func newResolver(root string, classes []Class, result *Resolution) *resolver {
	x := &resolver{root: root, result: result, byName: map[string]Class{}, bySimple: map[string][]Class{}, seen: map[string]bool{}, chosen: map[string]bool{}}
	for _, c := range classes {
		x.byName[c.Name] = c
		name := simpleName(c.Name)
		x.bySimple[name] = append(x.bySimple[name], c)
	}
	return x
}
func simpleName(name string) string { return name[strings.LastIndex(name, ".")+1:] }
func (x *resolver) reason(s string) { x.result.Reasons = append(x.result.Reasons, s) }
func (x *resolver) resolveChain(cur Class) []string {
	var args []string
	vars := map[string]string{}
	for {
		if x.seen[cur.Name] {
			x.reason("constructor inheritance cycle")
			break
		}
		x.seen[cur.Name] = true
		src, ok := x.readSource(cur)
		if !ok {
			break
		}
		x.overrides(src)
		x.hooks(src)
		nextArgs, nextVars, ok := x.constructor(src, cur, args, vars)
		if !ok {
			break
		}
		args, vars = nextArgs, nextVars
		next, ok := x.parent(cur)
		if !ok {
			break
		}
		cur = next
	}
	return args
}

func (x *resolver) parent(cur Class) (Class, bool) {
	if cur.Parent == "ModelCheckerTestCase" || cur.Parent == "PCalModelCheckerTestCase" {
		return Class{}, false
	}
	next, ok := findParent(cur, cur.Parent, x.byName, x.bySimple)
	if !ok {
		x.reason("parent not resolved: " + cur.Parent)
	}
	return next, ok
}

func (x *resolver) readSource(cur Class) (string, bool) {
	data, err := os.ReadFile(filepath.Join(x.root, filepath.FromSlash(cur.Source)))
	if err != nil {
		x.reason(err.Error())
		return "", false
	}
	return commentRE.ReplaceAllStringFunc(string(data), func(s string) string {
		if strings.HasPrefix(s, "//") || strings.HasPrefix(s, "/*") {
			return " "
		}
		return s
	}), true
}

func (x *resolver) overrides(src string) {
	for _, m := range methodName.FindAllStringSubmatch(src, -1) {
		name := m[1]
		if x.chosen[name] {
			continue
		}
		x.chosen[name] = true
		if !x.overrideValue(src, name) {
			x.reason("dynamic override: " + name)
		}
	}
}

func (x *resolver) overrideValue(src, name string) bool {
	for _, v := range returnMethod.FindAllStringSubmatch(src, -1) {
		if v[1] == name {
			return x.setOverride(name, v[2])
		}
	}
	return false
}

func (x *resolver) setOverride(name, v string) bool {
	switch name {
	case "doCoverage", "doDump", "checkDeadLock":
		return x.setBooleanOverride(name, v)
	case "getNumberOfThreads", "doCheckpoint":
		if _, err := strconv.Atoi(v); err != nil {
			return false
		}
		if name == "getNumberOfThreads" {
			x.result.Case.Workers = v
		} else {
			x.result.Checkpoint = v
		}
		return true
	}
	return false
}

func (x *resolver) setBooleanOverride(name, v string) bool {
	if v != "true" && v != "false" {
		return false
	}
	b := v == "true"
	switch name {
	case "doCoverage":
		x.result.Coverage = b
	case "doDump":
		x.result.Dump = b
	case "checkDeadLock":
		x.result.Deadlock = b
	}
	return true
}

func (x *resolver) hooks(src string) {
	for _, hook := range []string{"beforeSetUp", "setUp", "beforeTearDown", "tearDown", "assertExitStatus"} {
		if regexp.MustCompile(`\b(?:void|boolean)\s+` + hook + `\s*\(`).MatchString(src) {
			x.reason("custom harness hook: " + hook)
		}
	}
}

func (x *resolver) constructor(src string, cur Class, args []string, vars map[string]string) ([]string, map[string]string, bool) {
	return x.constructorDelegation(src, cur, args, vars, map[int]bool{})
}

func (x *resolver) constructorDelegation(src string, cur Class, args []string, vars map[string]string, seen map[int]bool) ([]string, map[string]string, bool) {
	match := x.selectConstructor(src, cur, args)
	if match == nil {
		x.reason("constructor not resolved: " + cur.Name)
		return nil, nil, false
	}
	if seen[match[0]] {
		x.reason("constructor this-delegation cycle")
		return nil, nil, false
	}
	seen[match[0]] = true
	params := splitJava(src[match[4]:match[5]])
	newVars := map[string]string{}
	for i, p := range params {
		fields := strings.Fields(p)
		if len(fields) > 0 {
			newVars[fields[len(fields)-1]] = javaValue(args[i], vars)
		}
	}
	end := callEnd(src, match[1])
	if end < 0 {
		x.reason("unterminated constructor call")
		return nil, nil, false
	}
	var next []string
	for _, a := range splitJava(src[match[1]:end]) {
		next = append(next, javaValue(a, newVars))
	}
	if strings.HasSuffix(strings.TrimSpace(src[match[0]:match[1]]), "this(") {
		return x.constructorDelegation(src, cur, next, newVars, seen)
	}
	return next, newVars, true
}

func (x *resolver) selectConstructor(src string, cur Class, args []string) []int {
	var match []int
	for _, m := range constructor.FindAllStringSubmatchIndex(src, -1) {
		if src[m[2]:m[3]] != simpleName(cur.Name) || len(splitJava(src[m[4]:m[5]])) != len(args) {
			continue
		}
		if match != nil {
			x.reason("ambiguous constructor overload")
			return nil
		}
		match = m
	}
	return match
}

func (r *Resolution) resolveArguments(args []string) {
	if len(args) == 0 {
		r.Reasons = append(r.Reasons, "missing spec")
		return
	}
	if v, err := strconv.Unquote(args[0]); err == nil {
		r.Case.Model = v
	} else {
		r.Reasons = append(r.Reasons, "dynamic spec: "+args[0])
	}
	for _, a := range args[1:] {
		r.resolveArgument(a)
	}
}

func (r *Resolution) resolveArgument(a string) {
	if parts := pathJoin.FindStringSubmatch(a); parts != nil {
		r.Path = filepath.Join(parts[1], parts[2])
		return
	}
	if strings.HasPrefix(a, `"`) {
		if v, e := strconv.Unquote(a); e == nil {
			r.Path = v
		} else {
			r.Reasons = append(r.Reasons, "invalid path: "+a)
		}
		return
	}
	if strings.HasPrefix(a, "new String[]") {
		r.resolveArray(a)
		return
	}
	if !strings.Contains(a, "ExitStatus.") {
		r.Reasons = append(r.Reasons, "dynamic constructor argument: "+a)
	}
}

func (r *Resolution) resolveArray(a string) {
	i, j := strings.Index(a, "{"), strings.LastIndex(a, "}")
	if i < 0 || j < i {
		r.Reasons = append(r.Reasons, "dynamic argument array: "+a)
		return
	}
	for _, item := range splitJava(a[i+1 : j]) {
		r.resolveArrayItem(item)
	}
}

func (r *Resolution) resolveArrayItem(item string) {
	v, e := strconv.Unquote(item)
	if e != nil {
		r.Reasons = append(r.Reasons, "dynamic CLI argument: "+item)
		return
	}
	if r.Translate {
		r.Case.TranslationArgs = append(r.Case.TranslationArgs, v)
	} else {
		r.Case.Args = append(r.Case.Args, v)
	}
}

func (r *Resolution) recordOmissions() {
	if r.Coverage {
		r.Reasons = append(r.Reasons, "coverage setup and assertions not reproduced")
	}
	if r.Dump {
		r.Reasons = append(r.Reasons, "dump setup and artifacts not reproduced")
	}
	if r.Translate {
		r.Reasons = append(r.Reasons, "PlusCal mapping and translation message assertions not reproduced")
	}
	r.Reasons = append(r.Reasons, "JUnit assertions, constructor side effects and Java test-specific overrides not fully reproduced")
}

// AssertedIDs extracts explicit recorder observations; callers must treat other
// assertion forms as omissions, not as covered observations.
func AssertedIDs(source, ecSource string) (bodies, presence []string, omissions []string) {
	codes := map[string]string{}
	source = maskJava(source, true)
	for _, m := range ecConst.FindAllStringSubmatch(maskJava(ecSource, true), -1) {
		codes[m[1]] = m[2]
	}
	for _, m := range recorded.FindAllStringSubmatch(source, -1) {
		id, ok := codes[m[2]]
		if !ok {
			omissions = append(omissions, fmt.Sprintf("unknown EC.%s", m[2]))
			continue
		}
		if m[1] == "recorded" {
			presence = append(presence, id)
		} else {
			bodies = append(bodies, id)
		}
	}
	numericBodies, numericOmissions := numericIDs(source, codes)
	bodies = append(bodies, numericBodies...)
	omissions = append(omissions, numericOmissions...)
	if strings.Contains(source, "assertZeroUncovered") || strings.Contains(source, "assertCoverage") {
		omissions = append(omissions, "coverage assertions")
	}
	return bodies, presence, omissions
}

func numericIDs(source string, codes map[string]string) (bodies, omissions []string) {
	numeric := regexp.MustCompile(`getRecordAsInt\s*\(\s*EC\.(\w+)`)
	for _, m := range numeric.FindAllStringSubmatch(source, -1) {
		if id, ok := codes[m[1]]; ok {
			bodies = append(bodies, id)
		} else {
			omissions = append(omissions, "unknown EC."+m[1])
		}
		omissions = append(omissions, "numeric predicate not evaluated: EC."+m[1])
	}
	return bodies, omissions
}
