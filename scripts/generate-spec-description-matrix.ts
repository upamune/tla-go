// bun scripts/generate-spec-description-matrix.ts /path/to/tla-ts > checker/testdata/upstream-spec-description-matrix.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const nexts=[
 'Step(1)',
 'Step(1) \\/ Step(2)',
 '\\E n \\in {1,2} : Step(n)',
 '\\E n \\in {x} : Step(n)',
 'LET n == 1 IN Step(n)',
 '(x = 0 /\\ x\' = 1) \\/ (x = 1 /\\ x\' = 2) \\/ (x = 2 /\\ x\' = 0)',
];
const properties=['[] (x < 3)','<> (x = 2)','[] [x\' \\in 0..2]_x','[]<>(x = 2)','(x = 0) ~> (x = 1)','\\A p \\in {1,2} : <>(x = p)'];
for(let i=0;i<120;i++){
 const next=nexts[i%nexts.length], property=properties[Math.floor(i/nexts.length)%properties.length];
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x \\in {0,1}\nExtra == x < 3\nStep(n) == x' = (x + n) % 3\nNext == ${next}\nSpec == Init /\\ Extra /\\ [][Next]_x\nInv == x < 3\nProp == ${property}\nBound == x < 3\nASSUME PrintT(TLCGet("spec"))\n====`;
 const config=(i%2?'SPECIFICATION Spec':'INIT Init NEXT Next')+(i%3===0?' INVARIANT Inv':'')+(i%4===0?' PROPERTY Prop':'')+(i%5===0?' CONSTRAINT Bound':'')+' CHECK_DEADLOCK FALSE';
 let printed:string[]=[];
 const r=checkSequentially({source,config,livenessCheck:'final',onPrint:(s:string)=>printed.push(s)});
 if(printed.length!==1)throw new Error(`expected one output ${i}: ${JSON.stringify(printed)}`);
 console.log(JSON.stringify({name:`spec-description-${i}`,source,config,expected:printed[0],status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
for(let i=0;i<12;i++){
 const spec=i%2?'Combined(1)':'Init /\\ [][Next]_x';
 const property=['Eventually(1)','Safety(2)','Eventually(1) /\\ Safety(2)'][i%3];
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nVARIABLE x\nInit == x = 0\nStep(n) == x' = (x + n) % 3\nNext == Step(1)\nCombined(n) == Init /\\ [][Step(n)]_x\nSpec == ${spec}\nEventually(n) == <> (x = n)\nSafety(n) == [] (x <= n)\nProp == ${property}\nASSUME PrintT(TLCGet("spec"))\n====`;
 const config='SPECIFICATION Spec PROPERTY Prop CHECK_DEADLOCK FALSE';
 let printed:string[]=[];
 const r=checkSequentially({source,config,livenessCheck:'final',onPrint:(s:string)=>printed.push(s)});
 if(printed.length!==1)throw new Error(`expected one alias output ${i}: ${JSON.stringify(printed)}`);
 console.log(JSON.stringify({name:`spec-description-alias-${i}`,source,config,expected:printed[0],status:r.status,generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}));
}
