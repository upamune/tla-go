package main

import (
	"fmt"
	"strings"

	"github.com/upamune/tla-go/checker"
	"github.com/upamune/tla-go/config"
)

// The SpecTE text below follows tlc2.TraceExplorer.writeSpecTEFiles and
// tlc2.output.SpecTraceExpressionWriter at the pinned upstream commit.
const (
	specTEIndent    = "    "
	specTETriIndent = specTEIndent + specTEIndent + specTEIndent
	specTEClosing   = "\n----\n"
)

type specTEVariable struct{ name, value string }

// specTEState is a printed trace state. A loop marker refers to the state
// with the given 1-based number, or to the current state when stuttering.
type specTEState struct {
	variables []specTEVariable
	backTo    int
	stutter   bool
}

func (s specTEState) marker() bool { return s.backTo > 0 || s.stutter }

// specTETrace converts a counterexample into the states Java TLC parses
// from its tool output, including a final back-to-state or stuttering
// marker for liveness counterexamples.
func specTETrace(trace []checker.TraceStep, loop *int) []specTEState {
	states := make([]specTEState, 0, len(trace)+1)
	for _, step := range trace {
		var variables []specTEVariable
		for _, name := range stateNames(step.State, step.StateOrder) {
			variables = append(variables, specTEVariable{name, stateValueText(step.State[name])})
		}
		states = append(states, specTEState{variables: variables})
	}
	if loop != nil && *loop == len(trace)-1 {
		states = append(states, specTEState{stutter: true})
	} else if loop != nil {
		states = append(states, specTEState{backTo: *loop + 1})
	}
	return states
}

// specTEBuffers returns the definitions inserted before the end of the
// module and the configuration appended to the copied configuration.
func specTEBuffers(states []specTEState, cfg config.Config, base, traceID string) (string, string) {
	var tla, cfgText strings.Builder
	writeSpecTEModelValues(&tla, &cfgText, cfg)
	subActions, initNext := specTEInitNext(&cfgText, states, base)
	tla.WriteString(subActions)
	writeSpecTETraceFunction(&tla, &cfgText, states, traceID)
	tla.WriteString(initNext)
	return tla.String(), cfgText.String()
}

// writeSpecTEModelValues declares the elements of set-valued constants as
// model values, in Java HashSet order.
func writeSpecTEModelValues(tla, cfgText *strings.Builder, cfg config.Config) {
	set := newJavaStringSet(16)
	for _, assignment := range cfg.Constants {
		if assignment.Value.Kind == "Set" && len(assignment.Args) == 0 {
			for _, element := range assignment.Value.Elements {
				set.add(configValueString(element))
			}
		}
	}
	names := set.values()
	if len(names) == 0 {
		return
	}
	cfgText.WriteString("CONSTANTS\n")
	for _, name := range names {
		fmt.Fprintf(cfgText, "%s%s = %s\n", specTEIndent, name, name)
	}
	cfgText.WriteString("\n")
	fmt.Fprintf(tla, "\nCONSTANTS %s\n\n", strings.Join(names, ", "))
}

func configValueString(v config.Value) string {
	switch v.Kind {
	case "Set":
		parts := make([]string, len(v.Elements))
		for i, element := range v.Elements {
			parts[i] = configValueString(element)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case "String":
		return fmt.Sprintf("%q", v.Text)
	}
	return v.Text
}

func specTEInitNext(cfgText *strings.Builder, states []specTEState, base string) (string, string) {
	cfgText.WriteString("\\* INIT definition\nINIT\n_SpecTEInit\n")
	var initNext strings.Builder
	initNext.WriteString("\\* VARIABLE TraceExp\n\n\\* TRACE INIT definition traceExploreInit\n_SpecTEInit ==\n")
	for _, variable := range states[0].variables {
		fmt.Fprintf(&initNext, "%s/\\ %s = (\n%s\n%s)\n", specTEIndent, variable.name, specTEReindent(variable.value, specTETriIndent), specTEIndent+specTEIndent)
	}
	initNext.WriteString("\\*     /\\ TraceExp = TRUE\n" + specTEClosing + "\n")
	cfgText.WriteString("\\* NEXT definition\nNEXT\n_SpecTENext\n")
	cfgText.WriteString("\\* Action Constraint definition\n\\* ACTION_CONSTRAINT\n\\* _SpecTEActionConstraint\n")
	subActions, names := specTESubActions(states, base)
	initNext.WriteString("\\* TRACE NEXT definition traceExploreNext\n_SpecTENext ==\n/\\ ")
	for i, name := range names {
		indent := specTEIndent
		if i == 0 {
			indent = " "
		}
		fmt.Fprintf(&initNext, "%s\\/ %s\n", indent, name)
	}
	initNext.WriteString("\\* /\\ TraceExp' = TraceExp\n\n\n")
	return subActions, initNext.String()
}

// specTESubActions writes one sub-action per step, resolving loop markers
// to the state they return to, and the action constraint selecting the
// sub-action for each level.
func specTESubActions(states []specTEState, base string) (string, []string) {
	var text, constraint strings.Builder
	constraint.WriteString("_SpecTEActionConstraint ==\n<<\n")
	current, single := states[0], len(states) == 1
	var names []string
	for i := 1; i < len(states) || single && i == 1; i++ {
		next := current
		if !single {
			next = resolveSpecTEState(states, states[i], current)
		}
		name := fmt.Sprintf("%s_sa_%d", base, len(names))
		names = append(names, name)
		writeSpecTESubAction(&text, len(names)-1, name, current, next, single)
		constraint.WriteString(name)
		if i+1 < len(states) {
			constraint.WriteString(",")
		}
		constraint.WriteString("\n")
		current = next
	}
	text.WriteString("\\* TRACE Action Constraint definition traceExploreActionConstraint\n")
	text.WriteString(constraint.String() + ">>[TLCGet(\"level\")]" + specTEClosing + "\n")
	return text.String(), names
}

func resolveSpecTEState(states []specTEState, state, current specTEState) specTEState {
	if state.backTo > 0 {
		return states[state.backTo-1]
	}
	if state.stutter {
		return current
	}
	return state
}

func writeSpecTESubAction(text *strings.Builder, index int, name string, current, next specTEState, single bool) {
	fmt.Fprintf(text, "\\* TRACE Sub-Action definition %d\n%s ==\n%s(\n", index, name, specTEIndent)
	writeSpecTEAssignments(text, current, "")
	if single {
		text.WriteString(specTEIndent + specTEIndent + "/\\ FALSE\n")
	}
	writeSpecTEAssignments(text, next, "'")
	text.WriteString(specTEIndent + ")\n\n")
}

func writeSpecTEAssignments(text *strings.Builder, state specTEState, prime string) {
	for _, variable := range state.variables {
		fmt.Fprintf(text, "%s/\\ %s%s = (\n%s\n%s)\n", specTEIndent+specTEIndent, variable.name, prime, specTEReindent(variable.value, specTETriIndent+specTEIndent), specTETriIndent)
	}
}

func writeSpecTETraceFunction(tla, cfgText *strings.Builder, states []specTEState, traceID string) {
	var records []string
	for _, state := range states {
		if state.marker() {
			continue
		}
		fields := make([]string, len(state.variables))
		for i, variable := range state.variables {
			fields[i] = variable.name + " |-> " + variable.value
		}
		records = append(records, "(["+strings.Join(fields, ",")+"])")
	}
	fmt.Fprintf(tla, "%s == \n<<\n%s\n>>%s\n\n", traceID, strings.Join(records, ",\n"), specTEClosing)
	fmt.Fprintf(cfgText, "CONSTANT\n_TETrace <- %s\n", traceID)
}

func specTEReindent(text, indent string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = indent + lines[i]
	}
	return strings.Join(lines, "\n")
}
