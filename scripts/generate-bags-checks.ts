// bun scripts/generate-bags-checks.ts /path/to/tla-ts > checker/testdata/upstream-bags-checks.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const model=`---- MODULE M ----
EXTENDS Bags, FiniteSets, Naturals
VARIABLE b
Init == b = EmptyBag
Add == \\E n \\in 1..2 : /\\ BagCardinality(b) < 3 /\\ b' = b (+) SetToBag({n})
Remove == \\E n \\in BagToSet(b) : b' = b (-) SetToBag({n})
Next == Add \\/ Remove
Spec == Init /\\ [][Next]_b
TypeOK == IsABag(b) /\\ BagCardinality(b) <= 3
Small == BagCardinality(b) < 2
Back == <> (b = EmptyBag)
====`;
const cases=[
 ['init-next',model,'INIT Init NEXT Next INVARIANT TypeOK'],
 ['specification',model,'SPECIFICATION Spec INVARIANT TypeOK'],
 ['invariant',model,'SPECIFICATION Spec INVARIANT Small'],
 ['eventually',model,'SPECIFICATION Spec PROPERTY Back'],
 ['bounded',model,'SPECIFICATION Spec INVARIANT TypeOK CONSTRAINT Small'],
];
for(const [name,source,config] of cases){
 const r=checkSequentially({source,config,livenessCheck:'final'});
 console.log(JSON.stringify({name,source,config,want:{status:r.status,name:r.violation?.name??'',generated:r.generatedStates,distinct:r.distinctStates,depth:r.depth}}));
}
