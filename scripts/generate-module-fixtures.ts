// bun scripts/generate-module-fixtures.ts /path/to/tla-ts > checker/testdata/upstream-modules.jsonl
import {join,resolve} from 'node:path';
import {readFileSync} from 'node:fs';
const root=resolve(process.argv[2]);
const {checkSequentially}=await import(join(root,'packages/tla-ts/src/check.ts'));
const {formatValue}=await import(join(root,'packages/tla-ts/src/values.ts'));
const fixture=(directory:string,name:string)=>readFileSync(join(root,'packages/tla-ts/tests/fixtures',directory,name),'utf8');
const mod=(name:string,body:string)=>`---- MODULE ${name} ----\n${body}\n====`;
const configs='INIT Init NEXT Next CHECK_DEADLOCK FALSE';
const cases=[
 ['simple',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = 0\nNext == x' = Inc(x) % 3"),{A:mod('A','EXTENDS Naturals\nInc(n) == n + 1')},configs],
 ['transitive',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = 0\nNext == x' = Twice(x) % 4"),{A:mod('A','EXTENDS B\nTwice(n) == Inc(Inc(n))'),B:mod('B','EXTENDS Naturals\nInc(n) == n + 1')},configs],
 ['diamond',mod('M',"EXTENDS A, B\nVARIABLE x\nInit == x = 0\nNext == x' = (IncA(x) + IncB(x)) % 5"),{A:mod('A','EXTENDS Base\nIncA(n) == Inc(n)'),B:mod('B','EXTENDS Base\nIncB(n) == Inc(n)'),Base:mod('Base','EXTENDS Naturals\nInc(n) == n + 1')},configs],
 ['imported-state',mod('M',"EXTENDS A\nInit == x = 0\nNext == Step\nInv == x <= Limit"),{A:mod('A',"EXTENDS Naturals\nCONSTANT Limit\nVARIABLE x\nStep == /\\ x < Limit /\\ x' = x + 1")},configs+' CONSTANT Limit = 3 INVARIANT Inv'],
 ['imported-predicates',mod('M','EXTENDS A'),{A:mod('A',"EXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == /\\ x < 3 /\\ x' = x + 1\nInv == x < 3")},configs+' INVARIANT Inv'],
 ['assumption',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = 0\nNext == x' = x"),{A:mod('A','ASSUME FALSE')},configs],
 ['imported-function',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = 0\nNext == x' = f[x]"),{A:mod('A','EXTENDS Naturals\nf[n \\in 0..2] == (n + 1) % 3')},configs],
 ['local-definitions',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = Base\nNext == x' = Step(x)"),{A:mod('A','LOCAL Hidden == 1\nBase == Hidden\nStep(n) == n')},configs],
 ['local-separated',mod('M',"EXTENDS A, B\nVARIABLE x\nInit == x = FromA\nNext == x' = FromB"),{A:mod('A','LOCAL Hidden == 1\nFromA == Hidden'),B:mod('B','LOCAL Hidden == 2\nFromB == Hidden')},configs],
 ['local-shadowing',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = Value\nNext == x' = F(x)"),{A:mod('A','F(C) == C\nLOCAL C == 1\nValue == C')},configs],
 ['extends-named-instance',mod('M',"EXTENDS Naturals, A\nVARIABLE x\nInit == x = 0\nNext == x' = (x + Value) % 3"),{A:mod('A','EXTENDS Naturals\nI == INSTANCE B\nValue == I!Inc(1)'),B:mod('B','EXTENDS Naturals\nInc(n) == n + 1')},configs],
 ['extends-local-instance',mod('M',"EXTENDS A\nVARIABLE x\nInit == x = Visible\nNext == x' = x"),{A:mod('A','LOCAL INSTANCE B\nVisible == Value'),B:mod('B','Value == 1')},'INIT Init NEXT Next'],
 ['extends-instance-dependency',mod('M',"EXTENDS A\nInit == v = 0\nNext == Step"),{A:mod('A','EXTENDS Base\nVARIABLE v\nI == INSTANCE B WITH N <- Bound, x <- v\nStep == I!Step'),Base:mod('Base','EXTENDS Naturals\nBound == 3'),B:mod('B',"EXTENDS Naturals\nCONSTANT N\nVARIABLE x\nStep == /\\ x < N /\\ x' = x + 1")},configs],
 ['inherited-nested',mod('M',"EXTENDS Holder\nVARIABLE x\nI == INSTANCE Held\nInit == x = I!Value\nNext == x' = x"),{Holder:mod('Holder','---- MODULE Held ----\nValue == 2\n====')},'INIT Init NEXT Next'],
 ['upstream-inherited-nested-assumption',mod('M',"EXTENDS Holder\nJ == INSTANCE Held\nASSUME J!Value = 2\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v"),{Holder:mod('Holder','---- MODULE Held ----\nValue == 2\n====')},'INIT Init NEXT Next'],
 ['upstream-nested-standard-shadow',mod('M',"---- MODULE Sequences ----\nMine == 1\n====\n---- MODULE User ----\nEXTENDS Sequences\nGot == Mine\n====\nU == INSTANCE User\nASSUME U!Got = 2\nVARIABLE v\nInit == v = 0\nNext == UNCHANGED v"),{},'INIT Init NEXT Next'],
 ['inherited-nested-private-parent',mod('M',"EXTENDS Holder\nVARIABLE x\nI == INSTANCE Held\nInit == x = I!Value\nNext == x' = x"),{Holder:mod('Holder','LOCAL Secret == 2\n---- MODULE Held ----\nValue == Secret\n====')},'INIT Init NEXT Next'],
 ['inherited-nested-transitive',mod('M',"EXTENDS Left, Right\nVARIABLE x\nI == INSTANCE Held\nInit == x = I!Value\nNext == x' = x"),{Left:mod('Left','EXTENDS Holder'),Right:mod('Right','EXTENDS Holder'),Holder:mod('Holder','---- MODULE Held ----\nValue == 2\n====')},'INIT Init NEXT Next'],
 ['inherited-nested-indirect',mod('M',"EXTENDS Consumer\nVARIABLE x\nInit == x = Visible\nNext == x' = x"),{Consumer:mod('Consumer','EXTENDS Holder\nI == INSTANCE Held\nVisible == I!Value'),Holder:mod('Holder','---- MODULE Held ----\nValue == 2\n====')},'INIT Init NEXT Next'],
 ['inherited-nested-state',mod('M',"EXTENDS Holder\nI == INSTANCE Held\nInit == v = 0\nNext == v' = (v + 1) % 3\nInv == I!Value = v + 1"),{Holder:mod('Holder','EXTENDS Naturals\nVARIABLE v\nLOCAL Bias == 1\n---- MODULE Held ----\nValue == v + Bias\n====')},'INIT Init NEXT Next INVARIANT Inv'],
 ['inherited-visible-in-nested',mod('M',"EXTENDS Holder\n---- MODULE Pair ----\nI == INSTANCE Held\nValue == I!Value\n====\nP == INSTANCE Pair\nVARIABLE x\nInit == x = P!Value\nNext == x' = x\nInv == x = 1"),{Holder:mod('Holder','---- MODULE Held ----\nValue == 1\n===='),Held:mod('Held','Value == 2')},'INIT Init NEXT Next INVARIANT Inv'],
 ['sibling-visible-in-nested',mod('M',"---- MODULE First ----\nValue == 1\n====\n---- MODULE Pair ----\nI == INSTANCE First\nValue == I!Value\n====\nP == INSTANCE Pair\nVARIABLE x\nInit == x = P!Value\nNext == x' = x\nInv == x = 1"),{},'INIT Init NEXT Next INVARIANT Inv'],
 ['nested-extends-inherited',mod('M',"EXTENDS Holder\n---- MODULE Pair ----\nEXTENDS Held\nGot == Value\n====\nP == INSTANCE Pair\nVARIABLE x\nInit == x = P!Got\nNext == x' = x\nInv == x = 2"),{Holder:mod('Holder','---- MODULE Held ----\nValue == 2\n===='),Held:mod('Held','Value == 7')},'INIT Init NEXT Next INVARIANT Inv'],
 ...['Holder, Held','Held, Holder'].map((names)=>[`inherited-nested-and-external-${names}`,mod('M',`EXTENDS ${names}\nVARIABLE x\nI == INSTANCE Held\nInit == x = I!Value\nNext == x' = x\nInv == x = 2 /\\ Value = 7`),{Holder:mod('Holder','---- MODULE Held ----\nValue == 2\n===='),Held:mod('Held','Value == 7')},'INIT Init NEXT Next INVARIANT Inv']),
 ['instance-let-shadow',mod('M',"EXTENDS Naturals\nVARIABLE x\nitem == 3\nI == INSTANCE A WITH C <- item, y <- x\nInit == x = 2\nNext == UNCHANGED x\nInv == I!P"),{A:mod('A','EXTENDS Naturals\nCONSTANT C\nVARIABLE y\nP == LET item == 1 IN C = item + y')},'INIT Init NEXT Next INVARIANT Inv'],
 ['upstream-nested-o',fixture('modules','O.tla'),{},fixture('modules','O.cfg')],
 ['upstream-nested-prime',fixture('modules','Github725e.tla'),{},fixture('modules','Github725e.cfg')],
 ['upstream-nested-fair',fixture('modules','Github725b.tla'),{},fixture('modules','Github725b.cfg')],
 ['upstream-nested-lambda',fixture('modules','Github725f.tla'),{},fixture('modules','Github725f.cfg')],
 ['upstream-nested-counters',fixture('modules','Counters.tla'),{Counter:fixture('modules','Counter.tla')},fixture('modules','Counters.cfg')],
 ['upstream-nested-modules',fixture('modules','NestedModules.tla'),{Clock:fixture('modules','Clock.tla')},fixture('modules','NestedModules.cfg')],
 ['nested-instance-spec',mod('M',"EXTENDS Naturals\nVARIABLE x\nI == INSTANCE Counter WITH Max <- 3, count <- x\nSpec == I!Spec\nP == <>(x >= 3)"),{Counter:fixture('modules','Counter.tla').replace(/====\s*$/, 'Spec == Init /\\ [][Next]_count /\\ WF_count(Next)\n====')},'SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE'],
 ['upstream-instances',fixture('instances','Instances.tla'),{Channel:fixture('instances','Channel.tla')},fixture('instances','Instances.cfg')],
 ['upstream-instances-violation',fixture('instances','Instances.tla'),{Channel:fixture('instances','Channel.tla')},fixture('instances','Violation.cfg')],
 ['upstream-instance-operators',fixture('instances','Operators.tla'),{Folding:fixture('instances','Folding.tla')},fixture('instances','Operators.cfg')],
 ['upstream-instance-operators-violation',fixture('instances','Operators.tla'),{Folding:fixture('instances','Folding.tla')},fixture('instances','OperatorsViolation.cfg')],
 ['upstream-instance-capture',fixture('instances','Capture.tla'),{CaptureInner:fixture('instances','CaptureInner.tla')},fixture('instances','Capture.cfg')],
 ['upstream-instance-binders',fixture('instances','Binders.tla'),{BindersInner:fixture('instances','BindersInner.tla')},fixture('instances','Binders.cfg')],
 ['upstream-instance-binders-violation',fixture('instances','Binders.tla'),{BindersInner:fixture('instances','BindersInner.tla')},fixture('instances','BindersViolation.cfg')],
 ['upstream-parameterized-spec',fixture('instances','ParameterizedSpec.tla'),{Counting:fixture('instances','Counting.tla')},fixture('instances','ParameterizedSpec.cfg')],
 ['upstream-parameterized-spec-violation',fixture('instances','ParameterizedSpec.tla'),{Counting:fixture('instances','Counting.tla')},fixture('instances','ParameterizedSpecViolation.cfg')],
 ...[['+','8'],['-','0'],['*','16']].map(([op,expected])=>[`instance-symbol-${op}`,mod('M',`EXTENDS Integers\nVARIABLE x\nI == INSTANCE SymbolsInner WITH Neg <- -., Op <- ${op}\nInit == x = 0\nNext == x' = x\nInv == I!Passed = ${expected} /\\ I!PassedNeg = -1`),{SymbolsInner:fixture('instances','SymbolsInner.tla')},'INIT Init NEXT Next INVARIANT Inv']),
 ['instance-parameter-forward',mod('M',"VARIABLE x\nP(k) == INSTANCE A WITH y <- k\nInit == x = 0\nNext == P(x)!Step"),{A:mod('A',"VARIABLE y\nSame == y' = y\nStep == Same")},'INIT Init NEXT Next'],
 ['instance-private',mod('M',"VARIABLE x\nI == INSTANCE A WITH y <- x\nInit == x = 0\nNext == I!Step"),{A:mod('A',"VARIABLE y\nLOCAL Hidden == y' = y\nStep == Hidden")},'INIT Init NEXT Next'],
 ['instance-extends',mod('M',"EXTENDS Naturals\nVARIABLE x\nI == INSTANCE A WITH v <- x\nInit == x = 0\nNext == I!Step\nInv == I!Inc(1) = 2"),{A:mod('A',"EXTENDS Base\nVARIABLE v\nStep == v' = Inc(v) % 3"),Base:mod('Base','EXTENDS Naturals\nInc(n) == n + 1')},'INIT Init NEXT Next INVARIANT Inv'],
 ['instance-external-chain',mod('M',"VARIABLE x\nI == INSTANCE A WITH u <- x\nInit == x = 0\nNext == I!Next"),{A:mod('A','VARIABLE u\nK == INSTANCE B WITH w <- u\nNext == K!Next'),B:mod('B','VARIABLE w\nJ == INSTANCE C WITH v <- w\nNext == J!Step'),C:mod('C',"VARIABLE v\nStep == v' = v")},'INIT Init NEXT Next'],
 ['nested-extends-parent',mod('M',"EXTENDS Naturals\nVARIABLE x\nI == INSTANCE A WITH v <- x\nInit == x = 0\nNext == I!Step"),{A:mod('A',"EXTENDS Base\nScale == 2\nVARIABLE v\n---- MODULE Inner ----\nStep == v' = (v + Inc(Scale)) % 3\n====\nI == INSTANCE Inner\nStep == I!Step"),Base:mod('Base','EXTENDS Naturals\nInc(n) == n + 1')},'INIT Init NEXT Next'],
 ['nested-instance-private',mod('M',"EXTENDS Naturals\nVARIABLE x\nI == INSTANCE A WITH Limit <- 3, v <- x\nInit == x = 0\nNext == I!Next"),{A:mod('A',"EXTENDS Naturals\nCONSTANT Limit\nVARIABLE v\n---- MODULE Inner ----\nCONSTANT By\nLOCAL Add(n) == n + By\nNext == /\\ v < Limit /\\ v' = Add(v)\n====\nSmall == INSTANCE Inner WITH By <- 1\nLarge == INSTANCE Inner WITH By <- 2\nNext == Small!Next \\/ Large!Next")},'INIT Init NEXT Next CHECK_DEADLOCK FALSE'],
 ['nested-local-instance',mod('M',"VARIABLE x\nI == INSTANCE A\nInit == x = I!Visible\nNext == x' = x"),{A:mod('A',"---- MODULE Inner ----\nValue == 1\n====\nLOCAL J == INSTANCE Inner\nVisible == J!Value")},'INIT Init NEXT Next'],
 ['instance-function-values',mod('M',"EXTENDS Naturals\nVARIABLE x\nP(k) == INSTANCE A WITH Base <- k, v <- x\nInit == x = 0\nNext == x' = (x + 1) % 3\nInv == /\\ P(1)!f[2] = x + 3 /\\ P(2)!f[2] = x + 4 /\\ P(1)!f[0] = x + 1"),{A:mod('A','EXTENDS Naturals\nCONSTANT Base\nVARIABLE v\nf[n \\in 0..3] == Base + v + n')},'INIT Init NEXT Next INVARIANT Inv'],
 ['instance-function-recursive',mod('M',"EXTENDS Naturals\nVARIABLE x\nP(k) == INSTANCE A WITH Base <- k\nInit == x = 0\nNext == x' = x\nInv == /\\ P(1)!f[4] = 5 /\\ P(2)!f[4] = 6"),{A:mod('A','EXTENDS Naturals\nCONSTANT Base\nf[n \\in 0..4] == IF n = 0 THEN Base ELSE f[n-1] + 1')},'INIT Init NEXT Next INVARIANT Inv'],
 ['instance-function-domain',mod('M',"EXTENDS Naturals\nVARIABLE x\nP(k) == INSTANCE A WITH Base <- k\nInit == x = 0\nNext == x' = x\nInv == /\\ P(1)!f[1] = 2 /\\ DOMAIN P(2)!f = 0..2"),{A:mod('A','EXTENDS Naturals\nCONSTANT Base\nf[n \\in 0..Base] == Base + n')},'INIT Init NEXT Next INVARIANT Inv'],
 ['instance-function-operator',mod('M',"EXTENDS Naturals\nVARIABLE x\nInc(n) == n + 1\nDouble(n) == n * 2\nP(F(_)) == INSTANCE A WITH G <- F\nInit == x = 0\nNext == x' = x\nInv == /\\ P(Inc)!f[2] = 3 /\\ P(Double)!f[2] = 4"),{A:mod('A','EXTENDS Naturals\nCONSTANT G(_)\nf[n \\in 0..3] == G(n)')},'INIT Init NEXT Next INVARIANT Inv'],
 ['upstream-scoped-standard',fixture('modules','Scoped.tla'),{},fixture('modules','Scoped.cfg')],
 ['upstream-scoped-root-assignment',fixture('modules','Scoped.tla'),{},fixture('modules','ScopedRoot.cfg')],
 ['upstream-scoped-instances',mod('M','EXTENDS Naturals\nCONSTANT Acc\nVARIABLE x\nMCBallot == 0..2\nV == INSTANCE Inner\nP(A) == INSTANCE Inner WITH Acc <- A\nInit == x = 0\nNext == UNCHANGED x\nInv == V!Safe /\\ P({3})!Safe /\\ V!Ballot = 0..2 /\\ x = 0'),{Inner:mod('Inner','EXTENDS Naturals\nCONSTANT Acc\nBallot == Nat\nSafe == \\A a \\in Acc, b \\in Ballot : b >= 0')},'INIT Init\nNEXT Next\nCONSTANT Acc = {1, 2}\nBallot <-[Inner] MCBallot\nINVARIANT Inv'],
 ['nested-scoped-parameters',mod('M','EXTENDS Naturals\nVARIABLE x\nR(n) == n + 10\nP(a) == INSTANCE Middle\nInit == x = 0\nNext == UNCHANGED x\nInv == P(2)!Q(3)!F(4) = 14'),{Middle:mod('Middle','EXTENDS Naturals\nQ(b) == INSTANCE Inner'),Inner:mod('Inner','EXTENDS Naturals\nF(n) == n + 1')},'INIT Init NEXT Next INVARIANT Inv CONSTANT F <- [Inner] R'],
];
const scopedAssignSource=mod('M',"EXTENDS Naturals\nVARIABLE x\nR(n) == n + 1\nI == INSTANCE Inner\nP(a) == INSTANCE Inner\nInit == x = 0\nNext == x' = I!F(x)\nInv == x /= 2 /\\ P(9)!F(0) = 1");
const scopedAssignModules={Inner:mod('Inner','EXTENDS Naturals\nF(n) == n + 1')};
for(const [name,assignment] of [
 ['value','F = [Inner] 1'],
 ['table','F(0) = [Inner] 1 F(1) = [Inner] 2 F(2) = [Inner] 2'],
 ['substitution-over-value','F = [Inner] 2 F <- [Inner] R'],
 ['substitution-over-table','F(0) = [Inner] 7 F <- [Inner] R'],
 ['instance-substitution','F = [Inner] 2 I!F <- R'],
 ['instance-assignment','F = [Inner] 2 I!F = 1'],
]) cases.push(['scoped-assignment-'+name,scopedAssignSource,scopedAssignModules,'INIT Init NEXT Next INVARIANT Inv CONSTANT '+assignment]);
const nestedAssignModules={Middle:mod('Middle','EXTENDS Naturals\nQ(b) == INSTANCE Inner'),Inner:mod('Inner','EXTENDS Naturals\nF(n) == n + 1')};
for(const [name,inv,assignment] of [
 ['outer-value','2','F = [Inner] 1 Q!F = [Middle] 2'],
 ['outer-over-inner-substitution','2','F <- [Inner] R Q!F = [Middle] 2'],
 ['outer-substitution','34','F = [Inner] 1 Q!F <- [Middle] S'],
 ['outer-table','34','F = [Inner] 1 Q!F(3,4) = [Middle] 34'],
]) cases.push(['nested-assignment-'+name,mod('M',`EXTENDS Naturals\nVARIABLE x\nR(n) == n + 10\nS(b,n) == b * 10 + n\nP(a) == INSTANCE Middle\nInit == x = 0\nNext == UNCHANGED x\nInv == P(2)!Q(3)!F(4) = ${inv}`),nestedAssignModules,'INIT Init NEXT Next INVARIANT Inv CONSTANT '+assignment]);
cases.push(['scoped-standard-value',mod('M',"EXTENDS Naturals\nVARIABLE x\nInit == x = 0\nNext == x' = x + 1\nInv == x \\in Nat"),{},'INIT Init NEXT Next INVARIANT Inv CONSTANT Nat = [Naturals] {0, 1}']);
for(const [name,source,modules,config] of cases){const r=checkSequentially({source,config,loadModule:(n:string)=>modules[n]});console.log(JSON.stringify({name,source,config,modules,want:{status:r.status,name:r.violation?.name??'',generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth,trace:r.violation?.trace.map(t=>({action:t.action??'',state:Object.fromEntries(Object.entries(t.state).map(([k,v])=>[k,formatValue(v)]))}))??null}}));}
