// bun scripts/generate-tlc-trace-matrix.ts /path/to/tla-ts > checker/testdata/upstream-tlc-trace-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
for(let i=0;i<36;i++){
 const n=2+i%4, start=i%3===0?'0':i%3===1?'{0, 1}':'{1}';
 const init=i%3===0?`x = ${start}`:`x \\in ${start}`;
 const next=[`x' \\in {x, (x + 1) % ${n}}`,`x' \\in {0, (x + 1) % ${n}}`,`x' \\in 0..${n-1}`][Math.floor(i/3)%3];
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC, TLCExt\nVARIABLE x\nInit == ${init}\nNext == ${next}\nBound == PrintT(<<"constraint", x, Trace>>)\nActionBound == PrintT(<<"action", x, x', Trace>>)\nInv == PrintT(<<"invariant", x, Trace>>)\nProp == [][PrintT(<<"property", x, x', Trace, Trace'>>)]_x\n====`;
 const config='INIT Init NEXT Next CONSTRAINT Bound ACTION_CONSTRAINT ActionBound INVARIANT Inv'+(i%2?' PROPERTY Prop':'')+' CHECK_DEADLOCK FALSE';
 const printed:string[]=[];
 const r=checkSequentially({source,config,onPrint:(s:string)=>printed.push(s)});
 console.log(JSON.stringify({name:`tlc-trace-${i}`,source,config,printed,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
