// bun scripts/generate-string-order-matrix.ts /path/to/tla-ts > checker/testdata/upstream-string-order-matrix.jsonl
// Rename a local `declare` function in a temporary copy because Bun strips its calls.
import {cp,mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {join,resolve} from 'node:path';
import {tmpdir} from 'node:os';
const dest=await mkdtemp(join(tmpdir(),'tla-go-string-order-'));
await cp(join(resolve(process.argv[2]),'packages/tla-ts/src'),dest,{recursive:true});
const path=join(dest,'tlc-order.ts'), original=await readFile(path,'utf8');
const patched=original.replace('const declare = (index: number, kind: SymbolKind)','const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "','declareUnit(index, "');
if(patched===original || patched.includes('declare(index,'))throw new Error('unexpected tlc-order source');
await writeFile(path,patched);
const {checkSequentially}=await import(join(dest,'check.ts'));
for(let i=0;i<96;i++){
 const names=i%2?'zz == 0\naa == 1':'aa == 1\nzz == 0';
 const items=i%3===0?'"aa", "zz", "mm"':i%3===1?'"zz", "mm", "aa"':'"mm", "aa", "zz"';
 const records=i%2?'[zz |-> 1, aa |-> 2]':'[aa |-> 2, zz |-> 1]';
 const initial=i%4===0?'P':`{${items}}`;
 const source=`---- MODULE M ----\nEXTENDS Naturals, FiniteSets, Sequences, TLC\nCONSTANT P\nVARIABLES names, pick\n${names}\nInit == names = ${initial} /\\ pick = CHOOSE s \\in names : TRUE\nNext == /\\ Cardinality(names) < 5 /\\ \\E n \\in {7, 3, 5} : /\\ names' = names \\cup {"q" \\o ToString(n + Cardinality(names))} /\\ pick' = CHOOSE t \\in names' : t # pick\nInv == pick # "q9"\nASSUME PrintT({${items}})\nASSUME PrintT(CHOOSE s \\in {${items}} : TRUE)\nASSUME PrintT(${records})\nASSUME PrintT({ToString("zz"), ToString("aa")})\nASSUME PrintT(P)\n====`;
 const config=`CONSTANT P = ${i%2?'{"mm","zz","aa"}':'{"zz","aa","mm"}'} INIT Init NEXT Next`+(i%2?' INVARIANT Inv':'')+' CHECK_DEADLOCK FALSE';
 const printed:string[]=[];
 try {
  const r=checkSequentially({source,config,onPrint:(s:string)=>printed.push(s)});
  console.log(JSON.stringify({name:`string-order-${i}`,source,config,printed,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
 }catch(e){console.log(JSON.stringify({name:`string-order-${i}`,source,config,error:(e as Error).message}));}
}
