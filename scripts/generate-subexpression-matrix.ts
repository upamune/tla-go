// bun scripts/generate-subexpression-matrix.ts /path/to/tla-ts > checker/testdata/upstream-subexpression-matrix.jsonl
import {cp, mkdtemp, readFile, writeFile} from 'node:fs/promises';
import {join, resolve} from 'node:path';
import {tmpdir} from 'node:os';

const dest = await mkdtemp(join(tmpdir(), 'tla-go-state-view-reduced-'));
await cp(join(resolve(process.argv[2]), 'packages/tla-ts/src'), dest, {recursive: true});
const path = join(dest, 'tlc-order.ts');
const original = await readFile(path, 'utf8');
const patched = original.replace('const declare = (index: number, kind: SymbolKind)', 'const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "', 'declareUnit(index, "');
if (patched === original || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path, patched);
const {checkSequentially} = await import(join(dest, 'check.ts'));

const expressions = [
 ['Add(2,3)!1','2'], ['Add(2,3)!2','3'], ['Add(2,3)!<<','2'], ['Add(2,3)!>>','3'],
 ['Both(2,3)!1','TRUE'], ['Both(2,3)!2','TRUE'], ['Both(2,3)!test','TRUE'],
 ['Tup(2,3)!1','2'], ['Tup(2,3)!2','3'],
 ['Set(2,3)!1','2'], ['Set(2,3)!2','3'],
 ['Pick(2)!1','FALSE'], ['Pick(2)!2','"two"'], ['Pick(2)!3','"other"'],
 ['Fun(3)!1','1..3'], ['Fun(3)!(2)','3'], ['Fun(3)!@','OpArg'],
 ['Filt(4)!1','1..4'], ['Filt(4)!(2)','TRUE'], ['Filt(4)!even(3)','FALSE'], ['Filt!@','FilterArg'],
 ['Quant(4)!1','1..4'], ['Quant(4)!(2)','TRUE'], ['Quant(4)!positive(2)','TRUE'],
 ['Wrapped(3)!1!inner(4)','7'], ['Wrapped(3)!local(4)','7'],
 ['Rec!1','2'], ['Rec!2','3'], ['Arr!1','<<1,2>>'], ['Arr!2','2'],
 ['Field!1','Rec'], ['Field!2','"a"'], ['Val!1','Fun2'], ['Val!2','<<1,3>>'],
];
for (const [i,[expr,expected]] of expressions.entries()) {
 const actual = expected === 'OpArg' ? `Apply2(${expr},3,2)` : expected === 'FilterArg' ? `Apply2(${expr},4,2)` : expr;
 const want = expected === 'OpArg' ? '3' : expected === 'FilterArg' ? 'TRUE' : expected;
 const source=`---- MODULE M ----\nEXTENDS Naturals\nVARIABLE x\nAdd(a,b)==a+b\nBoth(a,b)==/\\ test::a>0 /\\ b>0\nTup(a,b)==<<a,b>>\nSet(a,b)=={a,b}\nPick(a)==CASE a=0 -> "zero" [] a=2 -> "two" [] OTHER -> "other"\nFun(a)==[i \\in 1..a |-> i+1]\nFilt(a)=={i \\in 1..a : even(i):: i%2=0}\nQuant(a)==\\A i \\in 1..a : positive(i):: i>0\nWrapped(a)==LET local(n)==inner:: n+a IN local(2)\nRec==[a |-> 2,b |-> 3]\nArr==<<1,2>>[2]\nField==Rec.a\nFun2==[i \\in 1..2, j \\in 3..4 |-> i+j]\nVal==Fun2[1,3]\nApply2(Op(_,_),a,b)==Op(a,b)\nASSUME ${actual} = ${want}\nInit==x=0\nNext==UNCHANGED x\n====`;
 const config='INIT Init NEXT Next CHECK_DEADLOCK FALSE';
 try {let r=checkSequentially({source,config});console.log(JSON.stringify({name:`subexpression-${i}`,source,config,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth,queued:r.queuedStates,violation:r.violation,failure:r.failure,outDegrees:r.outDegrees}));}
 catch(e){console.log(JSON.stringify({name:`subexpression-${i}`,source,config,error:(e as Error).message}));}
}
