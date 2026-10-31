package main

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

// The copiers below follow tlc2.output.TLACopier, CFGCopier, and
// TLAMonolithCreator at the pinned upstream commit, including their
// line-based matching.
var (
	specTEClosingLine   = regexp.MustCompile(`^[=]+$`)
	specTELocalInstance = regexp.MustCompile(`^LOCAL INSTANCE ([^\s]+)\s*$`)
)

func specTEModuleHeader(name string) *regexp.Regexp {
	return regexp.MustCompile(`^([-]+ MODULE) ` + regexp.QuoteMeta(name))
}

// javaLines splits text as BufferedReader.readLine does.
func javaLines(text string) []string {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// copySpecTEModule renames the root module to SpecTE, adds TLC and Toolbox
// to each EXTENDS line, and inserts definitions before each closing line.
func copySpecTEModule(source, name string, extends []string, definitions string) string {
	header := specTEModuleHeader(name)
	var additions string
	if !slices.Contains(extends, "TLC") {
		additions += ", TLC"
	}
	if !slices.Contains(extends, "Toolbox") {
		additions += ", Toolbox"
	}
	var out strings.Builder
	inBody := false
	for _, line := range javaLines(source) {
		switch {
		case !inBody:
			if match := header.FindStringSubmatch(line); match != nil {
				line, inBody = match[1]+" SpecTE ----", true
			}
		case strings.HasPrefix(strings.TrimSpace(line), "EXTENDS"):
			line += additions
		case specTEClosingLine.MatchString(line):
			out.WriteString(definitions + "\n")
		}
		out.WriteString(line + "\n")
	}
	return out.String()
}

// copySpecTEConfig drops the SPECIFICATION, INIT, and NEXT entries and a
// previous generation timestamp, then appends the trace configuration.
func copySpecTEConfig(source, definitions string, now time.Time) string {
	var out strings.Builder
	skipNext := false
	for _, line := range javaLines(source) {
		if skipNext {
			skipNext = false
			continue
		}
		keep, skip := specTEConfigLine(line)
		skipNext = skip
		if keep {
			out.WriteString(line + "\n")
		}
	}
	out.WriteString(definitions + "\n")
	out.WriteString("\\* Generated on " + now.Format("Mon Jan 02 15:04:05 MST 2006") + "\n")
	return out.String()
}

// specTEConfigLine reports whether a configuration line is copied and
// whether the following line, the operand of a bare keyword, is skipped.
func specTEConfigLine(line string) (keep, skipNext bool) {
	for _, keyword := range []string{"SPECIFICATION", "INIT", "NEXT"} {
		if strings.HasPrefix(line, "\\* "+keyword) {
			return false, false
		}
	}
	trimmed := strings.TrimSpace(line)
	for _, keyword := range []string{"SPECIFICATION", "INIT", "NEXT"} {
		if trimmed == keyword {
			return false, true
		}
		if strings.HasPrefix(trimmed, keyword) {
			return false, false
		}
	}
	return !strings.HasPrefix(trimmed, "\\* Generated on "), false
}

// monolith rewrites the EXTENDS line of SpecTE and embeds the bodies of the
// non-standard modules it depends on, unless nomonolith was requested.
func (m specTEModules) monolith(text string, embed bool) string {
	vetoed := map[string]bool{}
	if embed {
		for _, module := range m.embedded {
			vetoed[module.name] = true
		}
	}
	header := specTEModuleHeader("SpecTE")
	var out strings.Builder
	inBody := false
	for _, line := range javaLines(text) {
		switch {
		case !inBody:
			inBody = header.MatchString(line)
			if !vetoedLocalInstance(line, vetoed) {
				out.WriteString(line + "\n")
			}
		case strings.HasPrefix(strings.TrimSpace(line), "EXTENDS"):
			m.writeExtends(&out, embed, vetoed)
		default:
			out.WriteString(line + "\n")
		}
	}
	return out.String()
}

func (m specTEModules) writeExtends(out *strings.Builder, embed bool, vetoed map[string]bool) {
	out.WriteString("EXTENDS " + strings.Join(m.extendsList(embed, vetoed), ", ") + "\n")
	if embed {
		for _, module := range m.embedded {
			writeEmbeddedModule(out, module, vetoed)
		}
	}
	out.WriteString(specTEFollows("SpecTE"))
}

// extendsList follows TLAMonolithCreator's HashSet: the transitive EXTENDS,
// without embedded modules when embedding, plus TLC and Toolbox.
func (m specTEModules) extendsList(embed bool, vetoed map[string]bool) []string {
	all := newJavaStringSet(16)
	for _, name := range m.extends {
		all.add(name)
	}
	values := all.values()
	set := newJavaStringSet(max(int(float32(len(values))/0.75)+1, 16))
	for _, name := range values {
		if !embed || !vetoed[name] {
			set.add(name)
		}
	}
	set.add("TLC")
	set.add("Toolbox")
	return set.values()
}

func writeEmbeddedModule(out *strings.Builder, module specTEEmbedded, vetoed map[string]bool) {
	out.WriteString(specTEFollows(module.name))
	header := specTEModuleHeader(module.name)
	inBody := false
	for _, line := range javaLines(module.source) {
		if !inBody {
			inBody = header.MatchString(line)
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "EXTENDS") {
			continue
		}
		if specTEClosingLine.MatchString(line) {
			return
		}
		if !vetoedLocalInstance(line, vetoed) {
			out.WriteString(line + "\n")
		}
	}
}

func vetoedLocalInstance(line string, vetoed map[string]bool) bool {
	match := specTELocalInstance.FindStringSubmatch(line)
	return match != nil && vetoed[match[1]]
}

func specTEFollows(name string) string {
	return "\n\\* \n\\*  " + name + " follows\n\\* \n\n"
}
