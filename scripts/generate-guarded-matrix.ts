// bun scripts/generate-guarded-matrix.ts /path/to/tla-ts > checker/testdata/upstream-guarded-matrix.jsonl
import {join, resolve} from 'node:path';
const root=resolve(process.argv[2]);
const {checkSequentially}=await import(join(root,'packages/tla-ts/src/check.ts'));
let seed=20261030;
const rand=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed>>>8;};
const set=(mask:number)=>`{${[0,1,2].filter((i)=>mask&(1<<i)).join(',')}}`;
for(let n=0;n<120;n++){
 const all=Array.from({length:9},(_,i)=>[Math.floor(i/3),i%3]);
 const edges=all.filter(([a,b])=>((a+1)%3===b)||rand()%2===0);
 const next=edges.map(([a,b])=>`(x = ${a} /\\ x' = ${b})`).join(' \\/ ');
 const trigger=set(rand()%8),required=set(rand()%8);
 const source=`---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == ${next}\nSpec == Init /\\ [][Next]_x\nP == [](x \\in ${trigger} => [](x \\in ${required}))\n====`;
 const config='SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE';
 const result=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name:`guarded-matrix-${n}`,source,config,status:result.status}));
}
