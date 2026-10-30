// bun scripts/generate-alias-matrix.ts /path/to/tla-ts > checker/testdata/upstream-alias-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const {formatValue}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/values.ts'));
for(let i=0;i<72;i++){
 const max=1+i%4, limit=1+Math.floor(i/4)%max, mode=Math.floor(i/4)%3;
 const alias=['Good','Scalar','Bad','Constant','Argument','Undefined'][Math.floor(i/12)%6];
 const source=`---- MODULE M ----\nEXTENDS Integers, TLC, TLCExt\nVARIABLE x\nInit == x = 0\nNext == x < ${max} /\\ x' = x + 1\nInv == x < ${limit}\nProp == <> (x = ${max+1})\nGood == [v |-> x, delta |-> x' - x, level |-> TLCGet("level"), path |-> Trace]\nScalar == x + 100\nBad == [v |-> 1 \\div (x - 1)]\nConstant == [c |-> 1]\nArgument(n) == [n |-> n]\nPost == PrintT(ToTrace(CounterExample))\n====`;
 const modeCfg=mode===0?'INVARIANT Inv':mode===1?'':'PROPERTY Prop CHECK_DEADLOCK FALSE';
 const config=`INIT Init NEXT Next ${modeCfg} ALIAS ${alias} POSTCONDITION Post`;
 const printed:string[]=[];
 const r=checkSequentially({source,config,livenessCheck:'final',onPrint:(s:string)=>printed.push(s)});
 const trace=(r.violation?.trace??[]).map((s:any)=>Object.entries(s.state).map(([key,v])=>[key,formatValue(v)]));
 console.log(JSON.stringify({name:`alias-${i}`,source,config,printed,trace,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth,loop:r.violation?.loopStart??null}));
}
