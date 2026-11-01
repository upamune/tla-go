package upstreamjunit

import (
	"regexp"
	"strconv"
	"strings"
)

// These rules describe the two pinned suite bases, not arbitrary subclasses of them.
func resolveSuite(root string, classes []Class, c Class, r Resolution) Resolution {
	x := newResolver(root, classes, &r)
	if c.Parent != "SuiteTestCase" && c.Parent != "SuiteETestCase" {
		x.reason("unsupported indirect suite inheritance")
		return r
	}
	src, ok := x.readSource(c)
	if !ok {
		return r
	}
	r.Case.Model, r.Path = strings.ToLower(simpleName(c.Name)), "suite"
	x.overrides(src)
	x.hooks(src)
	if regexp.MustCompile(`@(?:Before|BeforeClass)\b`).MatchString(maskJava(src, true)) {
		x.reason("custom harness hook: JUnit before method")
	}
	args, ok := suiteConstructor(x, src, c)
	if ok && c.Kind == "Suite" {
		suiteRegular(x, src, args)
	} else if ok {
		suiteError(x, args)
	}
	r.recordOmissions()
	return r
}

func suiteConstructor(x *resolver, src string, c Class) ([]string, bool) {
	name := regexp.QuoteMeta(simpleName(c.Name))
	decl := regexp.MustCompile(`\b` + name + `\s*\(`)
	if !decl.MatchString(maskJava(src, true)) {
		return nil, true // Java's implicit no-argument constructor.
	}
	m := x.selectConstructor(src, c, nil)
	if m == nil || !strings.HasSuffix(strings.TrimSpace(src[m[0]:m[1]]), "super(") {
		x.reason("constructor not resolved: " + c.Name)
		return nil, false
	}
	end := callEnd(src, m[1])
	if end < 0 {
		x.reason("unterminated constructor call")
		return nil, false
	}
	if !regexp.MustCompile(`^\s*;\s*}`).MatchString(src[end+1:]) {
		x.reason("constructor side effects not resolved")
	}
	return splitJava(src[m[1]:end]), true
}

func suiteRegular(x *resolver, src string, args []string) {
	values := []string{"2", "1", "0", "1"}
	if !validSuiteArity(len(args)) {
		x.reason("constructor unsupported suite arguments")
		return
	}
	if len(args) > 0 && !suiteValues(x, values, args[:4]) {
		return
	}
	if len(args) == 5 {
		x.reason("coverage expectation not reproduced")
		if !regexp.MustCompile(`^\s*"(?:\\.|[^"\\])*"(?:\s*\+\s*"(?:\\.|[^"\\])*")*\s*$`).MatchString(args[4]) {
			x.reason("dynamic suite coverage expectation")
		}
	}
	if regexp.MustCompile(`\bvoid\s+testSpec\s*\(`).MatchString(maskJava(src, true)) {
		return
	}
	x.result.Inherited = []AssertedObservation{
		{ID: "2186", Present: true},
		{ID: "2199", Present: true, Values: values[:3]},
		{ID: "2190", Present: true, Values: values[3:]},
		{ID: "1000", Present: false},
	}
}

func validSuiteArity(n int) bool { return n == 0 || n == 4 || n == 5 }

func suiteValues(x *resolver, values, args []string) bool {
	for i, a := range args {
		v, err := strconv.Unquote(a)
		if err != nil {
			x.reason("dynamic suite expectation: " + a)
			return false
		}
		values[i] = v
	}
	return true
}

func suiteError(x *resolver, args []string) {
	x.reason("suite error output interception and substring assertions not reproduced")
	if len(args) > 2 || (len(args) == 2 && (!strings.HasPrefix(args[0], "new String[]") || !strings.HasPrefix(args[1], "ExitStatus."))) {
		x.reason("constructor unsupported error suite overload")
		return
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "new String[]") {
			x.result.resolveArray(arg)
		} else if !regexp.MustCompile(`^ExitStatus\.\w+$`).MatchString(arg) {
			x.reason("dynamic constructor argument: " + arg)
		}
	}
}
