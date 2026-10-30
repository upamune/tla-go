// bun scripts/generate-tlc-ext-matrix.ts /path/to/tla-ts > checker/testdata/upstream-tlc-ext-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
for(let i=0;i<48;i++){
 const key=i%2===0?String(i%4):`"s:r${i%3}"`;
 const init=Math.floor(i/2)%3, delta=i%3+1, initial=i%3;
 const update=i%4<2?'LAMBDA old, new : old + new':'LAMBDA old, new : new';
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC, TLCExt\nVARIABLE x\nASSUME PrintT(<<TLCGetOrDefault(${key}, ${init}), TLCGetAndSet(${key}, ${update}, ${delta}, ${init}), TLCGetOrDefault(${key}, 99)>>)\nASSUME TLCDefer(PrintT("deferred") /\\ 1 \\div 0 = 0)\nInit == x = ${initial}\nNext == x < 3 /\\ x' = x + 1\nInv == AssertEq(TLCNoOp(x), ${i%4}) \\/ x >= 0\n====`;
 const config='INIT Init NEXT Next INVARIANT Inv CHECK_DEADLOCK FALSE';
 const printed:string[]=[];
 const r=checkSequentially({source,config,onPrint:(s:string)=>printed.push(s)});
 console.log(JSON.stringify({name:`tlc-ext-${i}`,source,config,printed,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
