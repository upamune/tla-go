// bun scripts/generate-printing-fixtures.ts /path/to/tla-ts > checker/testdata/upstream-printing.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const module=(body:string)=>`---- MODULE M ----\nEXTENDS Naturals, TLC\n${body}\n====`;
const cases=[
 ['assumptions',module('ASSUME PrintT("first")\nASSUME Print(1, TRUE)\nASSUME PrintT("last")'),''],
 ['short-circuit',module('ASSUME TRUE \\/ PrintT("hidden")\nASSUME FALSE => PrintT("also hidden")\nASSUME PrintT("shown")'),''],
 ['operator-argument',module('Apply(F(_), x) == F(x)\nASSUME Apply(PrintT, "called")'),''],
 ['initial',module('VARIABLE x\nInit == /\\ x = 0 /\\ PrintT("initial")\nNext == UNCHANGED x'),'INIT Init NEXT Next'],
 ['action',module('VARIABLE x\nInit == x = 0\nNext == IF x = 0 THEN /\\ PrintT("next") /\\ x\' = 1 ELSE /\\ PrintT("done") /\\ UNCHANGED x'),'INIT Init NEXT Next'],
 ['invariant',module('VARIABLE x\nInit == x = 0\nNext == x\' = 1\nInv == Print("value", x) >= 0'),'INIT Init NEXT Next INVARIANT Inv'],
];
for(const [name,source,config] of cases){
 const printed:string[]=[];
 const r=checkSequentially({source,config,onPrint:(line:string)=>printed.push(line)});
 console.log(JSON.stringify({name,source,config,printed,want:{status:r.status,name:r.violation?.name??'',generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}}));
}
