// bun scripts/generate-counter-example-matrix.ts /path/to/tla-ts > checker/testdata/upstream-counter-example-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
for(let i=0;i<24;i++){
 const max=2+i%4, initial=i%2, mode=Math.floor(i/2)%3, limit=1+i%max;
 const next=mode===2?`Step \\/ Back`:'Step';
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC, TLCExt\nVARIABLES x, y\nInit == x = ${initial} /\\ y = 10\nStep == x < ${max} /\\ x' = x + 1 /\\ UNCHANGED y\nBack == x = ${max} /\\ x' = ${initial} /\\ UNCHANGED y\nNext == ${next}\nInv == x < ${limit}\nProp == <> (x = ${max+1})\nPost == PrintT(CounterExample) /\\ PrintT(ToTrace(CounterExample))\n====`;
 const check=mode===0?'INVARIANT Inv':mode===1?'':'PROPERTY Prop';
 const config=`INIT Init NEXT Next ${check} POSTCONDITION Post`+(mode===2?' CHECK_DEADLOCK FALSE':'');
 const printed:string[]=[];
 const r=checkSequentially({source,config,livenessCheck:'final',onPrint:(s:string)=>printed.push(s)});
 console.log(JSON.stringify({name:`counter-example-${i}`,source,config,printed,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
