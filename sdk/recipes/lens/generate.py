#!/usr/bin/env python3
"""Reproducibly compile the packed-Recipes lens and local EVM probes."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
parser = argparse.ArgumentParser()
parser.add_argument("--check", action="store_true", help="verify committed artifacts without editing")
args = parser.parse_args()
with tempfile.TemporaryDirectory(prefix="merkle-recipe-lens-") as temporary:
    root = Path(temporary)
    (root / "src").mkdir()
    for name in ("RecipePrefixLens.sol", "LensProbe.sol"):
        (root / "src" / name).write_bytes((HERE / name).read_bytes())
    (root / "foundry.toml").write_text('[profile.default]\nsolc = "0.8.37"\nevm_version = "cancun"\noptimizer = true\noptimizer_runs = 200\nvia_ir = true\nbytecode_hash = "none"\n')
    # Foundry verifies/downloads the pinned compiler on a fresh CI host.
    subprocess.run(["forge", "build", "--skip", "test", "--root", temporary], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    for contract, source, output in (("RecipePrefixLens", "RecipePrefixLens.sol", "bytecode.json"), ("LensProbe", "LensProbe.sol", "LensProbe.json"), ("ZeroApprovalToken", "LensProbe.sol", "ZeroApprovalToken.json"), ("FixtureAutomaticRewards", "LensProbe.sol", "FixtureAutomaticRewards.json")):
        artifact = json.loads((root / "out" / source / (contract + ".json")).read_text())
        runtime = artifact["deployedBytecode"]["object"]
        result = {"compiler": artifact["metadata"]["compiler"]["version"],
                  "evmVersion": "cancun", "sourceSHA256": hashlib.sha256((HERE / source).read_bytes()).hexdigest(),
                  "runtimeSHA256": hashlib.sha256(bytes.fromhex(runtime.removeprefix("0x"))).hexdigest(),
                  "recipesRevision": "bd5e235899de5e175a0001a913d47596f6f4f801", "runtime": runtime}
        if contract != "RecipePrefixLens":
            result.update(abi=artifact["abi"], bytecode=artifact["bytecode"]["object"])
        text = json.dumps(result, indent=2)+"\n"
        path = HERE / output
        if args.check:
            if not path.exists() or path.read_text() != text:
                raise SystemExit("stale recipe lens artifact: " + output)
        else:
            path.write_text(text)
