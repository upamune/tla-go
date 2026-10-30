// bun scripts/generate-pretty-fixtures.ts /path/to/tla-ts > checker/testdata/upstream-pretty.jsonl
import {join,resolve} from 'node:path';
const {checkSequentially}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/check.ts'));
const expressions=[
 '[x \\in 1..10 |-> [y \\in 1..10 |-> y]]',
 '[x \\in 1..5 |-> [fielda |-> x, fieldb |-> 1..x]]',
 '[x \\in 1..3 |-> [y \\in 1..3 |-> [z \\in 1..3 |-> <<x, y, z>>]]]',
 '[fieldaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa |-> <<1, 2>>]',
 '[x \\in 2..9 |-> x * 100000000]',
 '<<"a\\"b", "cccccccccccccccccccccccccccccccccccccccccccc", "dddddddddddddddddddddddddddddd">>',
 '[x \\in 1..20 |-> x]',
 'SUBSET {"aaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccc"}',
];
for(const expr of expressions){
 const source=`---- MODULE M ----\nEXTENDS Naturals, TLC\nASSUME PrintT(${expr})\n====`;
 const printed:string[]=[];
 const result=checkSequentially({source,config:'',onPrint:(line:string)=>printed.push(line)});
 console.log(JSON.stringify({expr,printed,status:result.status}));
}
