// node sdk/providers/merkl/testdata/generate-frontend.mjs /absolute/frontend-checkout /path/frontend-guarded.json
// Uses the frontend's locked development dependencies; no RPC or source mutation.
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
const root = resolve(process.argv[2] ?? process.cwd());
const { createJiti } = await import(pathToFileURL(root + '/node_modules/jiti/lib/jiti.mjs').href);
const jiti = createJiti(import.meta.url, { alias: { '@': root + '/src' }, fsCache: false });
const { keccak256, encodeAbiParameters, parseAbiParameters, concatHex } = await jiti.import(root + '/node_modules/viem/_esm/index.js');
const base = root + '/src/shared/lib/merkle/operations/authoring/catalog/';
const { compilePermissions } = await jiti.import(base + 'compile.ts');
const { deploymentFor } = await jiti.import(base + 'deployments.ts');
const { merklRewards, MERKL_DISTRIBUTOR } = await jiti.import(base + 'types/merkl.ts');
const { RecipeContext } = await jiti.import(root + '/src/shared/lib/merkle/operations/authoring/recipes/resolve.ts');
const { recipeTemplate, canonicalParams } = await jiti.import(root + '/src/shared/lib/merkle/operations/authoring/recipes/registry.ts');
const address = value => '0x' + value.toString(16).padStart(40, '0');
const account = address(0x104), adapter = address(0x103), capability = address(0x105);
const tokens = [address(0x201), address(0x202)], amounts = [100n, 200n];
const leaves = tokens.map((token, index) => keccak256(encodeAbiParameters(parseAbiParameters('address,address,uint256'), [account, token, amounts[index]])));
const rewardRoot = keccak256(concatHex(BigInt(leaves[0]) < BigInt(leaves[1]) ? leaves : [...leaves].reverse()));
const zeroAddress = address(0), zeroHash = '0x' + '00'.repeat(32);
const compiled = await compilePermissions(tokens.map(token => ({ type: 'merkl_rewards', params: { token, proofDepth: '1' } })), {
  chainId: 1, account, adapter, vaultAsset: tokens[0], deployments: { ...deploymentFor(1), recipeCapability: capability },
  facts: {
    read: async (_target, signature, args) => signature.startsWith('getMerkleRoot') ? rewardRoot : signature.startsWith('claimed') ? [0n, 0n, zeroHash] : args[1] === zeroAddress ? zeroAddress : account,
    token: async () => ({ symbol: 'REWARD', decimals: 0 }), code: async () => '0x',
  },
}, [merklRewards]);
if (!compiled.root || compiled.issues.length) throw new Error(JSON.stringify(compiled.issues));
const recipeSet = {...compiled, account, adapter, chainId:1};
const recipeCtx = new RecipeContext(recipeSet, {...deploymentFor(1), recipeCapability: capability}, {aave:{}});
const template = recipeTemplate('claims');
const built = template.build(recipeCtx, canonicalParams(recipeCtx,template,{claims:compiled.permissions.filter(p=>p.type==='merkl_rewards').map(p=>p.id+':claim')}));
if(!built) throw new Error(JSON.stringify(recipeCtx.issues));
const fixture = {recipe:built.recipe,queries:built.queries,steps:built.steps,inputs:built.inputs,runtime:built.runtime, provenance: { generator: 'frontend-merkl-fixture.mjs',
    frontendRevision: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim(),
    merklSourceSha256: createHash('sha256').update(readFileSync(base + 'types/merkl.ts')).digest('hex'),
    facts: 'modeled topology, no chain reads' }, account, adapter, capability, distributor: MERKL_DISTRIBUTOR, rewardRoot,
  rewards: tokens.map((token, index) => ({ token, cumulative: amounts[index], proof: [leaves[1 - index]] })),
  root: compiled.root, calls: compiled.calls, requirements: compiled.requirements };
const json = JSON.stringify(fixture, (_key, value) => typeof value === 'bigint' ? value.toString() : value, 2) + '\n';
const output = resolve(process.argv[3] ?? 'frontend-merkl-fixture.json');
writeFileSync(output, json);
console.log(`Wrote ${output}: two current guarded Merkl calls, exact masks and proofs.`);
