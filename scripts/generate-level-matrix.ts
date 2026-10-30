// bun scripts/generate-level-matrix.ts /path/to/tla-ts > checker/testdata/upstream-level-matrix.jsonl
import {join,resolve} from 'node:path';
import {cp,mkdtemp,readFile,writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
const dest=await mkdtemp(join(tmpdir(),'tla-go-level-'));
await cp(join(resolve(process.argv[2]),'packages/tla-ts/src'),dest,{recursive:true});
const path=join(dest,'tlc-order.ts');
const old=await readFile(path,'utf8');
const patched=old.replace('const declare = (index: number, kind: SymbolKind)','const declareUnit = (index: number, kind: SymbolKind)').replaceAll('declare(index, "','declareUnit(index, "');
if(patched===old || patched.includes('declare(index,')) throw new Error('unexpected tlc-order source');
await writeFile(path,patched);
const {checkSequentially}=await import(join(dest,'check.ts'));
let seed=20261105;
const rand=(n:number)=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return (seed>>>8)%n;};
const rel=(edges:number[][])=>edges.map(([a,b])=>`(x = ${a} /\\ x' = ${b})`).join(' \\/ ');
for(let i=0;i<320;i++) {
 const all=Array.from({length:9},(_,n)=>[Math.floor(n/3),n%3]);
 const edges=all.filter(([a,b])=>b===(a+1)%3||rand(2)===0);
 const n=rand(4)+1, target=rand(3), fair=Math.floor(i/8)%2===0?' /\\ WF_x(Next)':'';
 const forms=[`<>(TLCGet("level") >= ${n})`,`[]<>(TLCGet("level") >= ${n})`,`<>[](TLCGet("level") <= ${n})`,`<>(x = ${target} /\\ <>(TLCGet("level") >= ${n}))`, `(TLCGet("level") >= ${n}) ~> (x = ${target})`, `(x = ${target}) ~> (TLCGet("level") >= ${n})`, `[]<>(TLCGet("level") >= ${n}) => []<>(x = ${target})`, `<>(TLCGet("level") >= ${n} /\\ <>(x = ${target}))`];
 const prop=forms[i%forms.length];
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x \\in {0,1}\nNext == ${rel(edges)}\nSpec == Init /\\ [][Next]_x${fair}\nP == ${prop}\n====`;
 const config='SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE';
 const r=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name:`level-${i}`,source,config,status:r.status,violation:r.violation}));
}
