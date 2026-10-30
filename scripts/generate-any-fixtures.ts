// bun scripts/generate-any-fixtures.ts /path/to/tla-ts > checker/testdata/upstream-any.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const values=['Any','Nat','Int','STRING','0','TRUE','FALSE','"a"','{}','{1}','1..2','<<>>','<<1>>','[a |-> 1]'];
const expressions=values.flatMap(value=>[`Any = ${value}`,`${value} = Any`]);
for(const [index,expr] of expressions.entries()) {
 const source=`---- MODULE M ----\nEXTENDS Integers, TLC\nASSUME ${expr}\n====`;
 const r=checkSequentially({source,config:''});
 console.log(JSON.stringify({name:`any-${index}`,source,config:'',status:r.status,message:r.violation?.message??''}));
}
