// bun scripts/generate-stays-or-often-matrix.ts /path/to/tla-ts > checker/testdata/upstream-stays-or-often-matrix.jsonl
import {join,resolve} from 'node:path';
const root=resolve(process.argv[2]);
const {checkSequentially}=await import(join(root,'packages/tla-ts/src/check.ts'));
let seed=20261104;
const rand=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed>>>8;};
const set=(mask:number)=>`{${[0,1,2].filter(i=>mask&(1<<i)).join(',')}}`;
const rel=(edges:number[][])=>edges.map(([a,b])=>`(x = ${a} /\\ x' = ${b})`).join(' \\/ ');
for(let n=0;n<480;n++){
 const all=Array.from({length:9},(_,i)=>[Math.floor(i/3),i%3]);
 const edges=all.filter(([a,b])=>((a+1)%3===b)||rand()%2===0);
 const actions=[0,1].map(()=>{const selected=all.filter(([a,b])=>a!==b&&rand()%2===0);return selected.length?selected:[[0,1]]});
 const fair=n%4<2?'':` /\\ ${rand()%2?'WF':'SF'}_x(A0)`;
 const assume=n%4===0?'':n%4===1?` /\\ []<>(x \\in ${set(rand()%8)})`:' /\\ []<><<A1>>_x';
 const stays=`<>[](x \\in ${set(rand()%8)})`;
 const recurrence=n%2===0?`[]<>(x \\in ${set(rand()%8)})`:'[]<><<A0>>_x';
 const formula=n%8<4?`${stays} \\/ ${recurrence}`:`${recurrence} \\/ ${stays}`;
 const source=`---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == ${rel(edges)}\n${actions.map((a,i)=>`A${i} == ${rel(a)}`).join('\n')}\nSpec == Init /\\ [][Next]_x${fair}${assume}\nP == ${formula}\n====`;
 const config='SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE';
 const result=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name:`stays-or-often-${n}`,source,config,status:result.status}));
}
