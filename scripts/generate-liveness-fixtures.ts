// bun scripts/generate-liveness-fixtures.ts /path/to/tla-ts > checker/testdata/upstream-liveness.jsonl
import {join,resolve} from 'node:path';
import {readFileSync} from 'node:fs';
const root=resolve(process.argv[2]);
const {checkSequentially}=await import(join(root,'packages/tla-ts/src/check.ts'));
const wrap=(body:string)=>`---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\n${body}\n====`;
const model=(prop:string)=>wrap(`Init == x = 0\nNext == /\\ x < 2 /\\ x' = x + 1\nP == ${prop}`);
const temporal=readFileSync(join(root,'packages/tla-ts/tests/fixtures/temporal/Temporal.tla'),'utf8');
const selected=temporal.split('\nWeakSpec')[0]+'\n'+temporal.split('\n').filter(line=>/^(WeakSpec|StrongSpec|OftenTop|EventuallyStaysTop|EventuallyBounded|OftenFlipped|Mixed) ==/.test(line)).join('\n')+'\n====';
const liveDir=join(root,'packages/tla-ts/tests/fixtures/liveness');
const upstreamLive=readFileSync(join(liveDir,'Liveness.tla'),'utf8');
const fair=wrap(`Init == x = 0
Move == /\\ x = 0 /\\ x' = 1
Back == /\\ x = 1 /\\ x' = 0
Enter == /\\ x = 1 /\\ x' = 2
Return == /\\ x = 2 /\\ x' = 0
Next == Move \\/ Back \\/ Enter \\/ Return
Base == Init /\\ [][Next]_x
Weak == Base /\\ WF_x(Move) /\\ WF_x(Enter)
Strong == Base /\\ WF_x(Move) /\\ SF_x(Enter)
Solo == Base /\\ SF_x(Enter)
Bound == x' # 2
Reach == <>(x = 2)
Often == []<>(x = 2)
Stays == <>[](x # 2)
Bounded == <>[](x < 3)`);
const quantified=wrap(`Init == x = 0
A(i,j) == /\\ x = 0 /\\ x' = IF i = j THEN 1 ELSE 2
Back == /\\ x > 0 /\\ x' = 0
Next == (\\E i,j \\in 1..2 : A(i,j)) \\/ Back
Base == Init /\\ [][Next]_x
Alias == \\A <<i,j>> \\in {<<1,1>>,<<1,2>>} : SF_x(A(i,j))
Tuples == Base /\\ Alias
Nested == Base /\\ \\A i \\in 1..2 : \\A j \\in 1..i : WF_x(A(i,j))
Inline == Base /\\ \\A i \\in 1..2 : WF_x(IF i = 1 THEN A(1,1) ELSE A(1,2))
Empty == Base /\\ \\A i \\in {} : WF_x(A(i,1))
Reach == <>(x = 2)
Often == []<>(x = 1)`);
const cases=[
 ['temporal-alias',wrap("Init == x = 0\nNext == x' = (x + 1) % 3\nOften == []<>(x = 2)\nP == Often"),'INIT Init NEXT Next PROPERTY P'],
 ['temporal-call',wrap("Init == x = 0\nNext == x' = (x + 1) % 3\nOften(n) == []<>(x = n)\nP == Often(2)"),'INIT Init NEXT Next PROPERTY P'],
 ['temporal-conjunction',wrap("Init == x = 0\nNext == x' = (x + 1) % 3\nAlwaysOK == [](x < 3)\nOften(n) == []<>(x = n)\nP == AlwaysOK /\\ Often(2)"),'INIT Init NEXT Next PROPERTY P'],
 ['eventually-fails',model('<>(x = 2)'),'INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE'],
 ['eventually-holds',model('<>(x = 0)'),'INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE'],
 ['often-fails',model('[]<>(x = 0)'),'INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE'],
 ['often-holds',model('[]<>(x < 3)'),'INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE'],
 ['stays-fails',model('<>[](x = 2)'),'INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE'],
 ['stays-holds',model('<>[](x < 3)'),'INIT Init NEXT Next PROPERTY P CHECK_DEADLOCK FALSE'],
 ['deadlock-precedes-liveness',model('<>(x = 2)'),'INIT Init NEXT Next PROPERTY P'],
 ['no-initial-state',wrap("Init == x \\in {}\nNext == x' = x\nP == <>(x = 2)"),'INIT Init NEXT Next PROPERTY P'],
 ['constrained',wrap("Init == x = 0\nNext == x' = x + 1\nBound == x < 2\nP == []<>(x < 2)"),'INIT Init NEXT Next PROPERTY P CONSTRAINT Bound CHECK_DEADLOCK FALSE'],
 ['upstream-temporal-often',selected,'SPECIFICATION Spec PROPERTY OftenTop'],
 ['upstream-temporal-stays',selected,'SPECIFICATION Spec PROPERTY EventuallyStaysTop'],
 ['upstream-temporal-true',selected,'SPECIFICATION Spec PROPERTY EventuallyBounded'],
 ...['WeakSpec','StrongSpec'].flatMap(spec=>['OftenTop','OftenFlipped','EventuallyStaysTop','EventuallyBounded'].map(prop=>[`upstream-temporal-${spec}-${prop}`,selected,`SPECIFICATION ${spec} PROPERTY ${prop}`])),
 ...['Spec','WeakSpec','StrongSpec'].map(spec=>[`upstream-temporal-${spec}-Mixed`,selected,`SPECIFICATION ${spec} PROPERTY Mixed`]),
 ...['Spec','WeakSpec','StrongSpec','QuantifiedSpec'].flatMap(spec=>['OnceTopThenNever','NeverTopAfterBottom'].map(prop=>[`upstream-temporal-original-${spec}-${prop}`,temporal,`SPECIFICATION ${spec} PROPERTY ${prop}`])),
 ...['Spec','WeakSpec','StrongSpec','AssumedSpec','QuantifiedSpec'].map(spec=>[`upstream-temporal-original-${spec}-VisitsInOrder`,temporal,`SPECIFICATION ${spec} PROPERTY VisitsInOrder`]),
 ...['Spec','WeakSpec','StrongSpec','QuantifiedSpec'].map(spec=>[`upstream-temporal-original-${spec}-TopThenBottom`,temporal,`SPECIFICATION ${spec} PROPERTY TopThenBottom`]),
 ...['Spec','WeakSpec','StrongSpec','QuantifiedSpec'].map(spec=>[`upstream-temporal-original-${spec}-OftenTopImpliesOftenFlip`,temporal,`SPECIFICATION ${spec} PROPERTY OftenTopImpliesOftenFlip`]),
 ...['Spec','WeakSpec','StrongSpec','QuantifiedSpec'].map(spec=>[`upstream-temporal-original-${spec}-TopLeadsToStaying`,temporal,`SPECIFICATION ${spec} PROPERTY TopLeadsToStaying`]),
 ...['Spec','WeakSpec','StrongSpec','QuantifiedSpec'].map(spec=>[`upstream-temporal-original-${spec}-SettlesOrFalls`,temporal,`SPECIFICATION ${spec} PROPERTY SettlesOrFalls`]),
 ...['OnceTopThenNever','NeverTopAfterBottom','EventuallyStaysTop','OftenTop','TopThenBottom','OftenTopImpliesOftenFlip','TopLeadsToStaying','SettlesOrFalls','OftenFlipped','FlipFollowsZero','EventuallyBounded','Mixed'].map(prop=>[`upstream-temporal-original-AssumedSpec-${prop}`,temporal,`SPECIFICATION AssumedSpec PROPERTY ${prop}`]),
 ...['Weak','Strong'].flatMap(spec=>['Reach','Often','Stays','Bounded'].map(prop=>[`fair-${spec}-${prop}`,fair,`SPECIFICATION ${spec} PROPERTY ${prop}`])),
 ['fair-strong-pruned',fair,'SPECIFICATION Solo PROPERTY Often ACTION_CONSTRAINT Bound'],
 ...['Unfair','Fair','Strong'].flatMap(spec=>['ReachesLimit','StaysAtLimit','NeverCritical','Served','TokenMoves'].map(prop=>[`upstream-liveness-${spec}-${prop}`,upstreamLive,readFileSync(join(liveDir,`${spec}-${prop}.cfg`),'utf8')])),
 ...['Tuples','Nested','Inline','Empty'].flatMap(spec=>['Reach','Often'].map(prop=>[`quantified-${spec}-${prop}`,quantified,`SPECIFICATION ${spec} PROPERTY ${prop} CHECK_DEADLOCK FALSE`])),
];
for(const [name,source,config] of cases){const r=checkSequentially({source,config,livenessCheck:'final'});console.log(JSON.stringify({name,source,config,want:{status:r.status,name:r.violation?.name??'',generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}}));}
