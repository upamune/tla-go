// bun scripts/generate-symmetry-general-matrix.ts /path/to/tla-ts > checker/testdata/upstream-symmetry-general-matrix.jsonl
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
const formulas = [
 '<>(x=a /\\ <>(bit=1))', '<>(bit=0 /\\ []<>(x=a))',
 '[]<>(x=a) => []<>(bit=1)', '<>[](x=a) \\/ []<>(bit=1)',
 '[](bit=0 => <>(x=b)) /\\ []<>(bit=1)', '[]<>(x=a) /\\ []<>(bit=1)',
];
for (let i=0; i<24; i++) {
 const names = i%2 ? '{a,b}' : '{b,a}', order=i%3 ? 'x,bit' : 'bit,x';
 const view=['','x','bit','<<x,bit>>'][i%4];
 const action=i%2 ? "x' \\in P /\\ bit' \\in {0,1}" : "x' \\in P /\\ bit' = 1-bit";
 for (let j=0; j<formulas.length; j++) {
  const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nCONSTANTS P, a, b\nVARIABLES ${order}\nInit == x \\in P /\\ bit = 0\nNext == ${action}\nSym == Permutations(P)\n${view ? `V == ${view}\n` : ''}Prop == ${formulas[j]}\n====`;
  const config=`CONSTANTS P = ${names} a = a b = b INIT Init NEXT Next SYMMETRY Sym${view ? ' VIEW V' : ''} PROPERTY Prop CHECK_DEADLOCK FALSE`;
  try {
   const r=checkSequentially({source,config});
   console.log(JSON.stringify({name:`symmetry-general-${i}-${j}`,source,config,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth,queued:r.queuedStates,failure:r.failure,violation:r.violation,outDegrees:r.outDegrees}));
  } catch(e) {console.log(JSON.stringify({name:`symmetry-general-${i}-${j}`,source,config,error:(e as Error).message}));}
 }
}
