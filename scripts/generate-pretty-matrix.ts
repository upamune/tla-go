// bun scripts/generate-pretty-matrix.ts /path/to/tla-ts > value/testdata/upstream-pretty-matrix.jsonl
import {join,resolve} from 'node:path';
const {prettyPrint}=await import(join(resolve(process.argv[2]),'packages/tla-ts/src/pretty-print.ts'));
let seed=20261104;
const rand=(n:number)=>{seed=(Math.imul(seed,1664525)+1013904223)>>>0;return (seed>>>8)%n;};
const atom=()=>['TRUE','FALSE','-12','1..20','"'+'x'.repeat(rand(60)+1)+'"','"🙂'.repeat(1)+'😀"','"a\\"b"','fieldabc','[1..2 -> 1..45]'][rand(9)];
const generate=(depth:number):string=>{
 if(depth===0||rand(5)===0)return atom();
 const kind=rand(5), count=rand(5)+1, children=Array.from({length:count},()=>generate(depth-1));
 if(kind===0)return `{${children.join(', ')}}`;
 if(kind===1)return `<<${children.join(', ')}>>`;
 if(kind===2)return `[${children.map((x,i)=>`field${i} |-> ${x}`).join(', ')}]`;
 if(kind===3)return `(${children.map((x,i)=>`${i+2} :> ${x}`).join(' @@ ')})`;
 return `SUBSET ${children[0]}`;
};
for(let i=0;i<300;i++){const input=generate(4);console.log(JSON.stringify({input,want:prettyPrint(input)}));}
