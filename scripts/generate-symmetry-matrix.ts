// bun scripts/generate-symmetry-matrix.ts /path/to/tla-ts > checker/testdata/upstream-symmetry-matrix.jsonl
// Bun treats calls to a function named `declare` as TypeScript declarations.
// Rename this local function in a temporary source copy to run the pinned code
// with the same variable-order semantics as the upstream Node test runner.
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
const root=resolve(process.argv[2]), dest=await mkdtemp(join(tmpdir(),'tla-go-upstream-oracle-'));
await cp(join(root,'packages/tla-ts/src'),dest,{recursive:true});
const orderPath=join(dest,'tlc-order.ts'), original=await readFile(orderPath,'utf8');
const patched=original.replace('const declare = (index: number, kind: SymbolKind)','const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "','declareUnit(index, "');
if(patched===original || patched.includes('declare(index,'))throw new Error('unexpected tlc-order source');
await writeFile(orderPath,patched);
const {checkSequentially}=await import(join(dest,'check.ts'));
const {getTlcVariableOrder}=await import(join(dest,'tlc-order.ts'));
for(let i=0;i<96;i++){
 const ps=i%2?'{p2, p1, p3}':'{p1, p2}', qs=i%3?'{q1,q2}':'{q2,q1,q3}';
 const vars=i%2?'x, y, seen':'seen, y, x';
 const view=['x','y','seen','<<x,y>>','<<y,seen>>','<<x, Cardinality(seen)>>'][Math.floor(i/2)%6];
 const source=`---- MODULE M ----\nEXTENDS Naturals, FiniteSets, TLC\nCONSTANTS P, Q\nVARIABLES ${vars}\nInit == x \\in P /\\ y \\in Q /\\ seen = {}\nChange == \\E p \\in P : x' = p /\\ UNCHANGED <<y,seen>>\nGrow == \\E p \\in P : seen' = seen \\cup {p} /\\ UNCHANGED <<x,y>>\nOther == \\E q \\in Q : y' = q /\\ UNCHANGED <<x,seen>>\nNext == Change \\/ Grow \\/ Other\nSpec == Init /\\ [][Next]_<<x,y,seen>>\nSym == Permutations(P) \\cup Permutations(Q)\nView == ${view}\nInv == Cardinality(seen) <= ${i%4}\nProp == <> (Cardinality(seen) = ${1+i%3})\n====`;
 const config=`CONSTANTS P = ${ps} Q = ${qs} ${i%3?'INIT Init NEXT Next':'SPECIFICATION Spec'} SYMMETRY Sym`+(i%4?' VIEW View':'')+(i%6===0?' INVARIANT Inv':'')+(i%5===0?' PROPERTY Prop':'')+' CHECK_DEADLOCK FALSE';
 try {
  const r=checkSequentially({source,config,livenessCheck:'final'});
  console.log(JSON.stringify({name:`symmetry-${i}`,source,config,order:getTlcVariableOrder(source),status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
 }catch(e){console.log(JSON.stringify({name:`symmetry-${i}`,source,config,error:(e as Error).message}));}
}
for(let i=0;i<24;i++){
 const names=i%2?'{b,a,c}':'{c,b,a}', vars=i%3?'x,y':'y,x';
 const view=['IF x = b THEN y ELSE a','IF y = b THEN x ELSE a','<<x = b,y>>','<<y = c,x>>'][i%4];
 const source=`---- MODULE M ----\nEXTENDS TLC\nCONSTANTS P, a, b, c\nVARIABLES ${vars}\nInit == x \\in P /\\ y \\in P\nNext == x' \\in P /\\ y' \\in P\nSym == Permutations(P)\nView == ${view}\n====`;
 const config=`CONSTANTS P = ${names} a = a b = b c = c INIT Init NEXT Next SYMMETRY Sym VIEW View CHECK_DEADLOCK FALSE`;
 try {
  const r=checkSequentially({source,config,livenessCheck:'final'});
  console.log(JSON.stringify({name:`symmetry-rank-${i}`,source,config,order:getTlcVariableOrder(source),status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
 }catch(e){console.log(JSON.stringify({name:`symmetry-rank-${i}`,source,config,error:(e as Error).message}));}
}
for(let i=0;i<36;i++){
 const vars=i%2?'x, bit':'bit, x', fair=i%3===0?' /\\ WF_vars(Change) /\\ WF_vars(Toggle)':'';
 const view=['x','bit','<<x,bit>>'][Math.floor(i/3)%3];
 const prop=['<> (x = a)','[]<> (x = a)','[]<><<Toggle>>_vars'][Math.floor(i/2)%3];
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANTS P, a, b\nVARIABLES ${vars}\nvars == <<x,bit>>\nInit == x \\in P /\\ bit = 0\nChange == \\E p \\in P : x' = p /\\ bit' = 1 - bit\nToggle == x' = x /\\ bit' = 1 - bit\nNext == Change \\/ Toggle\nSpec == Init /\\ [][Next]_vars${fair}\nSym == Permutations(P)\nView == ${view}\nProp == ${prop}\n====`;
 const config=`CONSTANTS P = {b,a} a = a b = b SPECIFICATION Spec SYMMETRY Sym`+(i%4?' VIEW View':'')+' PROPERTY Prop CHECK_DEADLOCK FALSE';
 try {
  const r=checkSequentially({source,config,livenessCheck:'final'});
  console.log(JSON.stringify({name:`symmetry-live-${i}`,source,config,order:getTlcVariableOrder(source),status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
 }catch(e){console.log(JSON.stringify({name:`symmetry-live-${i}`,source,config,error:(e as Error).message}));}
}
