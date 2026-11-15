// node sdk/providers/cowswap/testdata/generate-frontend.mjs /absolute/frontend-checkout /path/bounded-cow.json
// Uses locked frontend dependencies only; no RPC or source mutation.
import { createHash } from "node:crypto";
import { deepStrictEqual } from "node:assert";
import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
const root = resolve(process.argv[2]);
const { createServer } = await import(
  pathToFileURL(root + "/node_modules/vite/dist/node/index.js").href
);
const server = await createServer({
  configFile: false,
  root,
  resolve: { alias: { "@": root + "/src" } },
  server: { middlewareMode: true, watch: null },
});
try {
  const {
    domainSeparator,
    encodeFunctionData,
    erc20Abi,
    concatHex,
    keccak256,
  } = await import(
    pathToFileURL(root + "/node_modules/viem/_esm/index.js").href
  );
  const base = root + "/src/shared/lib/merkle/operations/";
  const { encodeCallTemplate } = await server.ssrLoadModule(
    "/src/shared/lib/merkle/operations/call-template.ts",
  );
  const { cowOrder } = await server.ssrLoadModule(
    "/src/shared/lib/merkle/operations/authoring/catalog/sign.ts",
  );
  const { signing } = await server.ssrLoadModule(
    "/src/shared/lib/merkle/operations/authoring/recipes/templates/signing.ts",
  );
  const { leafHash } = await server.ssrLoadModule(
    "/src/shared/lib/merkle/tree.ts",
  );
  const address = (n) => "0x" + n.toString(16).padStart(40, "0");
  const account = address(0x104),
    token = address(0x201),
    buyToken = address(0x202),
    reads = address(0x301);
  const settlement = "0x9008D19f58AAbD9eD0D60971565AA8510560ab41",
    spender = "0xC92E8bdf79f0507f65a392b0ab4667716BFE0110";
  const domain = domainSeparator({
    domain: {
      name: "Gnosis Protocol",
      version: "v2",
      chainId: 1,
      verifyingContract: settlement,
    },
  });
  const signData = encodeCallTemplate(
    cowOrder({ account, domain, sellToken: token, buyToken, summary: "Sign" }),
  ).data;
  const approvalData = encodeFunctionData({
    abi: erc20Abi,
    functionName: "approve",
    args: [spender, 0n],
  });
  const approval = {
    target: token,
    value: "zero",
    data: approvalData,
    dataMask: "0x" + "00".repeat(36) + "ff".repeat(32),
  };
  approval.hash = leafHash(approval);
  const config = {
    account,
    signData,
    price: { target: reads, data: "0xa035b1fe", offset: 0 },
    clock: { target: reads, data: "0xb80777ea", offset: 0 },
    minAmount: 1n,
    maxAmount: 1000n,
    priceDenominator: 1n,
    slippageBps: 100n,
    maxDuration: 300n,
    amountOffset: 260,
    outputOffset: 292,
    deadlineOffset: 324,
    deadlineBytes: 4,
    token,
    spender,
    approvalCall: approval.hash,
    compiler: { optimize: false },
  };
  const built = signing(config),
    sign = built.layout.authorization;
  const leaves = [sign, approval];
  const rootHash = keccak256(
    concatHex(
      leaves
        .map((l) => l.hash)
        .sort((a, b) => (BigInt(a) < BigInt(b) ? -1 : 1)),
    ),
  );
  leaves.forEach((leaf, i) => (leaf.proof = [leaves[1 - i].hash]));
  const fixture = {
    account,
    token,
    buyToken,
    reads,
    settlement,
    spender,
    config,
    recipe: built.recipe,
    root: rootHash,
    leaves,
    provenance: {
      generator: "sdk/providers/cowswap/testdata/generate-frontend.mjs",
      signingSourceSha256: createHash("sha256")
        .update(readFileSync(base + "authoring/recipes/templates/signing.ts"))
        .digest("hex"),
      facts: "modeled topology; no external chain or API calls",
    },
  };
  const json =
    JSON.stringify(
      fixture,
      (_key, value) => (typeof value === "bigint" ? value.toString() : value),
      2,
    ) + "\n";
  if (process.argv[3] === "--check") {
    for (const path of process.argv.slice(4))
      deepStrictEqual(
        JSON.parse(readFileSync(resolve(path))),
        JSON.parse(json),
        path,
      );
  } else {
    for (const path of process.argv.slice(3))
      writeFileSync(resolve(path), json);
  }
} finally {
  await server.close();
}
