package pcal

import "strings"

// builtInSymbols lists the TLA+ symbols recognized by PcalBuiltInSymbols.
var builtInSymbols = strings.Fields(`_ ASSUME ASSUMPTION AXIOM BOOLEAN CASE CONSTANT CONSTANTS
EXCEPT EXTENDS FALSE IF INSTANCE LOCAL MODULE OTHER STRING THEOREM TRUE VARIABLE VARIABLES
WITH WF_ SF_ >>_ ]_ ( [ { << ) } ] >> \A \forall \E \exists \AA \EE ~ \lnot \neg <> CHOOSE
ENABLED UNCHANGED SUBSET UNION DOMAIN ' ^+ ^* ^# => \cdot <=> \equiv ~> -+-> \subseteq
\subset \supset \supseteq \ll \gg \ \cap \intersect \cup \union /\ \/ \land \lor \X - + * /
^ | || & && ++ -- ** // ^^ |- |= -| =| <: :> := ::= (+) (-) \oplus \ominus (.) \odot (/)
\oslash (\X) \otimes \uplus \sqcap \sqcup \div \star \o \circ \bigcirc \bullet \in \notin = #
/= < > =< <= >= \prec \succ \preceq \succeq \sim \simeq \approx \doteq \asymp \sqsubset
\sqsupset \sqsubseteq \sqsupseteq \propto : -> |-> <- == ELSE THEN LET IN [] .. ... $ $$ ? ??
% %% ## @@ !! \times \leq \geq \mod \wr \cong ! ; , . -. @`)

type parenKind int

const (
	notParen parenKind = iota
	leftParen
	rightParen
)

var parenSymbols = map[string]parenKind{
	"(": leftParen, "[": leftParen, "{": leftParen, "<<": leftParen,
	")": rightParen, "}": rightParen, "]": rightParen, ">>": rightParen,
	">>_": rightParen, "]_": rightParen,
}

var (
	builtInSet    = map[string]bool{}
	builtInPrefix = map[string]bool{}
	stringChars   = map[uint16]bool{}
)

func init() {
	for _, sym := range builtInSymbols {
		builtInSet[sym] = true
		if isLetter(uint16(sym[0])) || (len(sym) > 1 && sym[0] == '\\' && isLetter(uint16(sym[1]))) {
			continue
		}
		for p := sym; p != ""; p = p[:len(p)-1] {
			builtInPrefix[p] = true
		}
	}
	legal := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ~!@#$%^&*()_-+={}[]|:;<,>.?/`'0123456789"
	for i := 0; i < len(legal); i++ {
		stringChars[uint16(legal[i])] = true
	}
}

func isBuiltInSymbol(s string) bool { return builtInSet[s] }

func isBuiltInPrefix(s string) bool { return builtInPrefix[s] }

func isStringChar(c uint16) bool { return stringChars[c] }

// isLetter, isDigit, and isSpace follow tla2tex.Misc.
func isLetter(c uint16) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '_'
}

func isDigit(c uint16) bool { return '0' <= c && c <= '9' }

func isSpace(c uint16) bool { return c == ' ' || c == '\f' || c == '\r' }
