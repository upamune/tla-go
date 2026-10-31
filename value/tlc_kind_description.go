package value

var tlcKindDescriptions = map[Kind]string{
	Invalid: "an undefined value", BooleanKind: "a Boolean value", IntegerKind: "an integer", StringKind: "a string", ModelKind: "a model value",
	SetKind: "a set of the form {e1, ... ,eN}", IntervalKind: "a set of the form i..j",
	LazyFunctionKind: "a function of the form  [x \\in S |-> expr]",
	FunctionSetKind:  "a set of the form [S -> T]", RecordSetKind: "a set of the form [d1 : S1, ... , dN : SN]",
	ProductSetKind: "a cartesian product", SubsetKind: "a set of the form SUBSET S",
	UnionKind: "a set of the form UNION  S", FilteredSetKind: "a set of the form {x \\in S : expr}",
}

var tlcSetOperationDescriptions = map[string]string{
	"\\": "a set of the form S \\ T", "\\cap": "a set of the form S \\cap T", "\\cup": "a set of the form S \\cup T",
}

// TLCKindDescription describes a value's representation in Java TLC errors.
func (v Value) TLCKindDescription() string {
	if v.kind == FunctionKind {
		return v.tlcFunctionKindDescription()
	}
	if v.kind == LazyFunctionKind && v.data.domain.IsEnumerable() {
		return "a function  of the form (d1 :> e1 @@ ... @@ dN :> eN)"
	}
	if v.tlcUnionEnumerated() {
		return tlcKindDescriptions[SetKind]
	}
	if v.kind == SetOperationKind {
		return tlcSetOperationDescriptions[v.data.text]
	}
	if description, ok := tlcKindDescriptions[v.kind]; ok {
		return description
	}
	return "a special set constant"
}

func (v Value) tlcFunctionKindDescription() string {
	if v.IsTuple() {
		return "a tuple"
	}
	if v.IsRecord() {
		return "a record"
	}
	return "a function  of the form (d1 :> e1 @@ ... @@ dN :> eN)"
}
