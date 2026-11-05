// node generate-frontend.mjs /absolute/frontend-checkout
// Reproduces the permission and single-call recipe without network access.
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
const root = resolve(process.argv[2]);
const { createJiti } = await import(pathToFileURL(root + '/node_modules/jiti/lib/jiti.mjs').href);
const jiti = createJiti(import.meta.url, { alias: { '@': root + '/src' }, fsCache: false });
const base = root + '/src/shared/lib/merkle/';
const { threeFGuardedClaim } = await jiti.import(base + 'operations/authoring/catalog/three-f-guarded-claim.ts');
const { encodeCallTemplate } = await jiti.import(base + 'operations/authoring/catalog/encode.ts');
const { deploymentFor } = await jiti.import(base + 'operations/authoring/catalog/deployments.ts');
const { leafHash } = await jiti.import(base + 'verify.ts');
const { encodeRecipe, MANAGE_PARAMS, Operation } = await jiti.import(base + 'operations/recipe-abi.ts');
const { encodeAbiParameters } = await jiti.import(root + '/node_modules/viem/_esm/index.js');
const account = '0x' + 'bb'.repeat(20);
const factory = deploymentFor(1).threeF.requestsFactory;
const spec = threeFGuardedClaim(account, factory);
const call = encodeCallTemplate(spec);
const hash = leafHash(spec.target, 0n, call.data, call.dataMask);
const recipe = encodeRecipe([], [{operation: Operation.Manage, data: encodeAbiParameters(MANAGE_PARAMS,[1n,[hash],[[]]])}]);
const fixture = { account, factory, chainId:1, recipe, root:hash,
  leaves:[{hash,target:account,value:'0',data:call.data,dataMask:call.dataMask,proof:[]}],
  provenance:{frontendRevision:execFileSync('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8'}).trim(),
    sourceSha256:createHash('sha256').update(readFileSync(base+'operations/authoring/catalog/three-f-guarded-claim.ts')).digest('hex')},
};
writeFileSync(resolve(dirname(fileURLToPath(import.meta.url)),'frontend-claim.json'),JSON.stringify(fixture,null,2)+'\n');
