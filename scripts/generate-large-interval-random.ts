// bun scripts/generate-large-interval-random.ts /path/to/tla-ts > checker/testdata/upstream-large-interval-random.jsonl
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

const ranges = ['0..2147483646', '0..2147483647', '(-2147483647)..0', '(-2147483647-1)..2147483647'];
for (const range of ranges) {
 for (const seed of [0n,1n,-42n,8006642976694192746n]) {
  for (const count of [0,1,5]) {
   const source=`---- MODULE M ----\nEXTENDS Integers, Randomization, TLC\nVARIABLE x\nInit==x=0\nNext==UNCHANGED x\nASSUME PrintT(RandomSubset(${count},${range}))\n====`;
   const config='INIT Init NEXT Next CHECK_DEADLOCK FALSE';
   const printed=[];
   try {const r=checkSequentially({source,config,seed,onPrint:line=>printed.push(line)});console.log(JSON.stringify({name:`range-${range}-${seed}-${count}`,source,config,seed:seed.toString(),printed,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));}
   catch(e){console.log(JSON.stringify({name:`range-${range}-${seed}-${count}`,source,config,seed:seed.toString(),error:(e as Error).message}));}
  }
 }
}
for (const seed of [0n,1n,-42n,8006642976694192746n]) {
 const source='---- MODULE M ----\nEXTENDS Integers, Randomization, TLC\nVARIABLE x\nInit==x=0\nNext==UNCHANGED x\nASSUME PrintT(RandomSubset(0,0..2147483647))\nASSUME PrintT(RandomSubset(5,(-2147483647-1)..2147483647))\nASSUME PrintT(RandomSubset(5,(-2147483647)..0))\n====';
 const config='INIT Init NEXT Next CHECK_DEADLOCK FALSE';
 const printed=[];
 try {const r=checkSequentially({source,config,seed,onPrint:line=>printed.push(line)});console.log(JSON.stringify({name:`sequential-${seed}`,source,config,seed:seed.toString(),printed,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));}
 catch(e){console.log(JSON.stringify({name:`sequential-${seed}`,source,config,seed:seed.toString(),error:(e as Error).message}));}
}
