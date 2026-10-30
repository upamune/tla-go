// bun scripts/generate-guarded-fairness-matrix.ts /path/to/tla-ts > checker/testdata/upstream-guarded-fairness-matrix.jsonl
import {join, resolve} from 'node:path';
const root=resolve(process.argv[2]);
const {checkSequentially}=await import(join(root,'packages/tla-ts/src/check.ts'));
let seed=20261031;
const rand=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed>>>8;};
const set=(mask:number)=>`{${[0,1,2].filter((i)=>mask&(1<<i)).join(',')}}`;
const rel=(edges:number[][])=>edges.map(([a,b])=>`(x = ${a} /\\ x' = ${b})`).join(' \\/ ');
for(let n=0;n<120;n++){
 const all=Array.from({length:9},(_,i)=>[Math.floor(i/3),i%3]);
 const edges=all.filter(([a,b])=>((a+1)%3===b)||rand()%2===0);
 const actions=[0,1].map(()=>{const selected=all.filter(([a,b])=>a!==b&&rand()%2===0);return selected.length?selected:[[0,1]]});
 const fairness=actions.map((_,i)=>`${rand()%2?'WF':'SF'}_x(A${i})`).join(' /\\ ');
 const source=`---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == ${rel(edges)}\n${actions.map((a,i)=>`A${i} == ${rel(a)}`).join('\n')}\nSpec == Init /\\ [][Next]_x /\\ ${fairness}\nP == [](x \\in ${set(rand()%8)} => [](x \\in ${set(rand()%8)}))\n====`;
 const config='SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE';
 const result=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name:`guarded-fair-matrix-${n}`,source,config,status:result.status}));
}
