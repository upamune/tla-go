// bun scripts/generate-fairness-matrix.ts /path/to/tla-ts > checker/testdata/upstream-fairness-matrix.jsonl
import {join,resolve} from 'node:path';
const root=resolve(process.argv[2]);
const {checkSequentially}=await import(join(root,'packages/tla-ts/src/check.ts'));
let seed=20261030;
const rand=()=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return seed>>>8;};
const rel=(edges:number[][])=>edges.map(([a,b])=>`(x = ${a} /\\ x' = ${b})`).join(' \\/ ');
for(let n=0;n<120;n++){
 const all=Array.from({length:9},(_,i)=>[Math.floor(i/3),i%3]);
 const edges=all.filter(([a,b])=>((a+1)%3===b)||rand()%3===0);
 const actions=[0,1].map(()=>{const selected=all.filter(([a,b])=>a!==b&&rand()%2===0);return selected.length?selected:[[0,1]]});
 const fair=actions.map((_,i)=>`${rand()%2?'WF':'SF'}_x(A${i})`).join(' /\\ ');
 const props=['<>(x = 2)','[]<>(x = 2)','<>[](x # 2)','(x = 1) ~> (x = 2)','\\A i \\in {0,1} : (x = i) ~> (x = 2)',"[]<><<x' = 2>>_x","[]<><<x = 1 /\\ x' = 0>>_x"];
 const prop=props[rand()%props.length];
 const source=`---- MODULE M ----\nVARIABLE x\nInit == x = 0\nNext == ${rel(edges)}\n${actions.map((a,i)=>`A${i} == ${rel(a)}`).join('\n')}\nSpec == Init /\\ [][Next]_x /\\ ${fair}\nP == ${prop}\n====`;
 const config='SPECIFICATION Spec PROPERTY P CHECK_DEADLOCK FALSE';
 const result=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name:`fair-matrix-${n}`,source,config,status:result.status}));
}
