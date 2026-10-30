// bun scripts/generate-tlc-trace-liveness.ts /path/to/tla-ts > checker/testdata/upstream-tlc-trace-liveness.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
for(let i=0;i<24;i++){
 const max=1+i%3, initial=i%2, goal=1+(Math.floor(i/2)%4), mode=i%3;
 const next=mode===0?`x' \\in 0..${max}`:mode===1?`x' = (x + 1) % ${max+1}`:`x' \\in {x, (x + 1) % ${max+1}}`;
 const source=`---- MODULE M ----\nEXTENDS Naturals, Sequences, TLCExt\nVARIABLE x\nInit == x = ${initial}\nNext == ${next}\nProp == <> (Len(Trace) >= ${goal})\n====`;
 const config='INIT Init NEXT Next PROPERTY Prop CHECK_DEADLOCK FALSE';
 const r=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name:`trace-liveness-${i}`,source,config,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
