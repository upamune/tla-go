// Run with Bun and the tla-ts checkout at the commit listed in README.md:
// bun scripts/generate-parser-fixtures.ts /path/to/tla-ts > syntax/parser/testdata/upstream-examples.json
import { join, resolve } from 'node:path';
const {parseModule} = await import(join(resolve(process.argv[2]), 'packages/tla-ts/src/parser.ts'));
const examples = [
 ['by-proof',`---- MODULE M ----\nA == 1 + 1 = 2\nTHEOREM T == A PROOF BY ONLY A DEF A\n====`],
 ['theorems',`---- MODULE M ----\nTHEOREM T == 1 = 1 OBVIOUS\nLEMMA 2 = 2 PROOF OMITTED\nTHEOREM 3 = 3\n====`],
 ['labels',`---- MODULE M ----\nF == lab :: 1 + 2\nG == lab(x, y) :: x = y\n====`],
 ['recursion',`---- MODULE M ----\nEXTENDS Naturals\nRECURSIVE Sum(_), _ @@ _\nSum(n) == IF n = 0 THEN 0 ELSE n + Sum(n - 1)\na @@ b == a + b\nFactorial[n \\in Nat] == IF n = 0 THEN 1 ELSE n * Factorial[n - 1]\nL == LET RECURSIVE F(_) F(n) == IF n = 0 THEN 0 ELSE F(n - 1) IN F(2)\n====`],
 ['cases',`---- MODULE M ----\nApply(F(_), a) == F(a)\nC(x) == CASE x = 0 -> 1 [] x = 1 -> 2 [] OTHER -> 3\nD == Apply(LAMBDA n : n + 1, 2)\n====`],
 ['selectors',`---- MODULE M ----\nA(x) == x + 1\nP == A( + )\nQ == I(1)!Foo(2)\nR == A!1\nS == A!<<\nT == A!(1)\n====`],
 ['updates',`---- MODULE M ----\nVARIABLE f\nNext == f' = [f EXCEPT ![1] = @ + 1, !.foo[2, 3] = 0]\n====`],
 ['instances',`---- MODULE Root ----\nEXTENDS Naturals\n---- MODULE Inner ----\nCONSTANT N\nVARIABLE x\nFoo == x + N\n====\nINSTANCE Inner WITH N <- 1\nLOCAL INSTANCE Inner WITH N <- 2\nI(v) == INSTANCE Inner WITH x <- v, N <- 3\nZ == LET J == INSTANCE Inner WITH N <- 4 IN J!Foo\n====`],
 ['counter',`---- MODULE Counter ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nSmall == x < 3\n====`],
 ['ranges',`---- MODULE Ranges ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nNext == x' = x\nIfError == IF x THEN 1 ELSE 2\nDomainError == DOMAIN x\nFieldError == f.c\nAndError == /\\ x = 0\n            /\\ x\nInfixError == (x + 1) \\cup {2}\nParen == (x = 0)\n====`],
 ['precedence',`---- MODULE M ----\nCONSTANTS P(_), C\nVARIABLES x, y\nF(a, b) == a + b * 2\nG == ~(x = 0) => y \\in {1, 2}\nH == <<x, y>>\nJ == F(x,y)\nK == f[x, y]\nL == x /\\ y /\\ C\n====`],
 ['literal',`---- MODULE M ----\nS == "~"\nD == 1.25\nA == \\hFF\nU == -2 + 3\n====`],
 ['let-fairness',`---- MODULE M ----\nVARIABLE x\nASSUME TRUE\nASSUME Nonnegative == x >= 0\nY == LET A == 1 B(n) == n + A IN B(x)\nSpec == WF_x(x = 1) /\\ SF_x(x = 2)\n====`],
 ['constructs',`---- MODULE M ----\nVARIABLE x\nC == [name |-> x, b |-> 2]\nR == [name: S, b: T]\nF == [x \\in S |-> x + 1]\nT == [S -> T]\nSpec == [][x = 1]_x\nA == <<x = 1>>_x\n====`],
 ['quantifiers',`---- MODULE M ----\nQ == \\A x, y \\in S, z \\in T : x = z\nU == \\EE x, y : x = y\nF == {x \\in S : x > 0}\nG == {x + 1 : x \\in S}\nC == CHOOSE x \\in S : x = 2\nT == \\E <<x, y>> \\in S : x = y\n====`],
 ['fairness',`---- MODULE M ----\nVARIABLE x\nNext == ENABLED (x = 0)\nTemporal == [](x = 0)\nUnchanged == UNCHANGED x\n====`],
];
const pos=(p:any)=>({Line:p.line,Column:p.column});
const range=(r:any)=>({Module:r.module,Start:pos(r.start),End:pos(r.end)});
const loc=(l:any)=>l?({...pos(l),Range:l.range ? range(l.range) : {Module:"",Start:{Line:0,Column:0},End:{Line:0,Column:0}}}):({Line:0,Column:0,Range:{Module:"",Start:{Line:0,Column:0},End:{Line:0,Column:0}}});
function expr(e:any):any {
 let text='',args:any[]=[];
 switch(e.kind){
 case 'Number':case 'Decimal':text=e.text;break;case 'String':text=e.value;break;case 'Identifier':text=e.name;break;
 case 'ModuleFact':text=e.name;break;case 'Subexpression':break;case 'OperatorReference':text=e.operator;break;case 'Apply':text=e.operator;args=e.args;break;case 'And':case 'Or':args=e.operands;break;
 case 'Tuple':case 'SetEnumeration':args=e.elements;break;case 'Product':args=e.factors;break;
 case 'Prime':case 'Unchanged':case 'Enabled':case 'Always':case 'Eventually':args=[e.expression];break;
 case 'If':args=[e.condition,e.consequent,e.alternative];break;
 case 'Forall':case 'Exists':case 'UnboundedQuantifier':case 'Choose':args=[e.body];break;case 'SetFilter':args=[e.predicate];break;case 'SetMap':args=[e.expression];break;
 case 'Label':text=e.name;args=[e.body];break;case 'Case':args=e.other?[e.other]:[];break;case 'Lambda':args=[e.body];break;case 'Except':args=[e.function];break;case 'ExceptAt':break;case 'Let':args=[e.body];break;case 'WeakFairness':case 'StrongFairness':args=[e.subscript,e.action];break;case 'FunctionConstruction':args=[e.body];break;case 'FunctionSet':args=[e.domain,e.range];break;case 'BoxAction':case 'AngleAction':args=[e.action,e.subscript];break;case 'Record':case 'RecordSet':break;
 case 'FunctionApplication':args=[e.function,...e.args];break;case 'FieldAccess':text=e.field;args=[e.record];break;
 default:throw new Error('unsupported kind '+e.kind);
 }
 const bound=(b:any)=>({Names:b.pattern.kind==='Tuple'?b.pattern.names:[b.pattern.name],Tuple:b.pattern.kind==='Tuple',Set:b.set?expr(b.set):null});
 let bounds:any[]=[];if(e.bounds)bounds=e.bounds.map(bound);if(e.kind==='SetFilter')bounds=[bound(e.bound)];if(e.kind==='Choose')bounds=[bound({pattern:e.pattern,set:e.set})];
 return {Kind:e.kind,Text:text,Bulleted:!!e.isBulleted,Quantifier:e.quantifier??'',Temporal:!!e.isTemporal,Names:(e.names??e.params)?.length?(e.names??e.params):null,ArgumentCounts:e.argumentCounts??null,Selectors:e.selectors?.length?e.selectors.map((s:any)=>({Kind:s.kind,Name:s.name??'',Operand:s.operand!==undefined?String(s.operand):'',Args:s.args?.length?s.args.map(expr):null})):null,Bounds:bounds.length?bounds:null,Fields:e.fields?e.fields.map((f:any)=>({Name:f.name,Value:expr(f.value)})):null,Arms:e.arms?.length?e.arms.map((a:any)=>({Condition:expr(a.condition),Body:expr(a.body)})):null,Updates:e.updates?e.updates.map((u:any)=>({Path:u.path.length?u.path.map((s:any)=>({Kind:s.kind,Name:s.name??'',Args:s.args?.length?s.args.map(expr):null})):null,Value:expr(u.value)})):null,Definitions:e.definitions?.length?e.definitions.map(def):null,Instances:e.instances?.length?e.instances.map(instance):null,Location:loc(e.location),Args:args.length?args.map(expr):null};
}
function declaration(d:any){return {Name:d.name,Arity:d.arity,Location:loc(d.location)}};
function def(d:any){return {Kind:d.kind,Recursive:d.isRecursive,Bounds:d.bounds?.length?d.bounds.map((b:any)=>({Names:b.pattern.kind==='Tuple'?b.pattern.names:[b.pattern.name],Tuple:b.pattern.kind==='Tuple',Set:expr(b.set)})):null,Name:d.name,Params:d.params?.length?d.params.map(declaration):null,Body:expr(d.body),Local:d.isLocal,Location:loc(d.location),BodyRange:d.bodyRange?range(d.bodyRange):{Module:'',Start:{Line:0,Column:0},End:{Line:0,Column:0}}}};
const emptyRange={Module:'',Start:{Line:0,Column:0},End:{Line:0,Column:0}};
function instance(i:any):any{return {Name:i.name??'',Params:i.params.length?i.params.map(declaration):null,Module:i.module,Substitutions:i.substitutions.length?i.substitutions.map((s:any)=>({Name:s.name,Expression:expr(s.expression)})):null,Local:i.isLocal,Location:loc(i.location),InstanceRange:range(i.instanceRange),DefinitionRange:i.definitionRange?range(i.definitionRange):emptyRange}}
export function projectModule(m:any):any{return {Name:m.name,Extends:m.extends.length?m.extends:null,Constants:m.constants.length?m.constants.map(declaration):null,Variables:m.variables.length?m.variables.map(declaration):null,Definitions:m.definitions.length?m.definitions.map(def):null,Instances:m.instances.length?m.instances.map(instance):null,Modules:m.modules.length?m.modules.map(projectModule):null,RecursiveDeclarations:m.units.flatMap((u:any)=>u.kind==='Recursive'?u.declarations:[]).length?m.units.flatMap((u:any)=>u.kind==='Recursive'?u.declarations:[]).map(declaration):null,Theorems:m.theorems.length?m.theorems.map((t:any)=>({Name:t.name??'',Statement:expr(t.statement),Proof:t.proof?{Kind:t.proof.kind,Only:!!t.proof.isOnly,Facts:t.proof.facts?.length?t.proof.facts.map(expr):null,Defs:t.proof.defs?.length?t.proof.defs.map(expr):null,Keyword:range(t.proof.keyword),Location:loc(t.proof.location)}:null,Location:loc(t.location)})):null,Assumptions:m.assumptions.length?m.assumptions.map((a:any)=>({Name:a.name??'',Expression:expr(a.expression),Axiom:a.isAxiom,Location:loc(a.location)})):null,Location:loc(m.location)}}
if (import.meta.main) {
 const data=examples.map(([name,source])=>({name,source,want:projectModule(parseModule(source))}));
 console.log(JSON.stringify(data,null,2));
}
