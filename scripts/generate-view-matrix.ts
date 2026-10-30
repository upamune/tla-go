// bun scripts/generate-view-matrix.ts /path/to/tla-ts > checker/testdata/upstream-view-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const props=['<> (x = 2)','[]<> (x = 2)','[]<><<B>>_vars','[][x\' <= 2]_vars'];
for(let i=0;i<96;i++){
 const max=2+i%2, init=i%3===0?'{0,1}':'{0}', view=i%4===0?'x % 2':i%4===1?'x':i%4===2?'<<x, y>>':'y';
 const property=props[Math.floor(i/4)%4], fair=i%3===0?' /\\ WF_vars(A) /\\ WF_vars(B)':'';
 const source=`---- MODULE M ----\nEXTENDS Naturals\nVARIABLES x, y\nvars == <<x, y>>\nInit == x = 0 /\\ y \\in ${init}\nA == x < ${max} /\\ x' = x + 1 /\\ y' = (y + 1) % 2\nB == x' = x /\\ y' = (y + 1) % 2\nNext == A \\/ B\nSpec == Init /\\ [][Next]_vars${fair}\nView == ${view}\nInv == x <= ${max} /\\ y \\in {0, 1}\nProp == ${property}\n====`;
 const config=`${i%2?'SPECIFICATION Spec':'INIT Init NEXT Next'} VIEW View`+(i%3===1?' INVARIANT Inv':'')+(i%5===0?'':' PROPERTY Prop')+' CHECK_DEADLOCK FALSE';
 try {
  const r=checkSequentially({source,config,livenessCheck:'final'});
  console.log(JSON.stringify({name:`view-${i}`,source,config,status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
 }catch(e){console.log(JSON.stringify({name:`view-${i}`,source,config,error:(e as Error).message}));}
}
