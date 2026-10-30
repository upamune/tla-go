// bun scripts/generate-possible-matrix.ts /path/to/tla-ts > checker/testdata/upstream-possible-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
let seed=20261106;
const rand=(n:number)=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return (seed>>>8)%n;};
const set=(mask:number)=>`{${[0,1,2].filter(i=>mask&(1<<i)).join(',')}}`;
const rel=(edges:number[][])=>edges.map(([a,b])=>`(x = ${a} /\\ x' = ${b})`).join(' \\/ ');
for(let i=0;i<240;i++){
 const all=Array.from({length:9},(_,n)=>[Math.floor(n/3),n%3]);
 const edges=all.filter(([a,b])=>b===(a+1)%3||rand(2)===0);
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x \\in {0,1}\nNext == ${rel(edges)}\nFirst == x \\in ${set(rand(8))}\nSecond == x \\in ${set(rand(8))}\nLevels == TLCGet("level") >= ${rand(4)+1}\nMove == (x \\in ${set(rand(8))}) /\\ (x' \\in ${set(rand(8))})\nBound == x \\in ${set(rand(8))}\nActionBound == x' \\in ${set(rand(8))}\nInv == x \\in ${set(rand(8))}\nPost == TLCGet("distinct") >= 1\n====`;
 const possible=i%2?'Move Second Levels First':'Levels First Move Second';
 const config=`INIT Init NEXT Next _POSSIBLE ${possible} CHECK_DEADLOCK FALSE${i%3===0?' CONSTRAINT Bound':''}${i%5===0?' ACTION_CONSTRAINT ActionBound':''}${i%4===0?' INVARIANT Inv':''}${i%7===0?' POSTCONDITION Post':''}`;
 const r=checkSequentially({source,config});
 console.log(JSON.stringify({name:`possible-${i}`,source,config,status:r.status,nameOf:r.violation?.name??'',message:r.violation?.message??'',counts:r.violation?.counts??'',unwitnessed:r.violation?.unwitnessed??null,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
