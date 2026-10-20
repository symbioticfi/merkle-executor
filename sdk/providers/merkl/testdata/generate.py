#!/usr/bin/env python3
"""Compile the unmodified pinned Multicall3 source and local Merkl getter fixture.

Install solc 0.8.12 in Foundry's compiler cache first. No external chain is used.
The production ABI is deliberately restricted to aggregate3.
"""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
SDK = HERE.parents[2]
UPSTREAM_SHA256 = "2054218939d3fa0f52f8ce1a33658d570a550671f63197356ee5744f7e188b1e"
if hashlib.sha256((HERE / "Multicall3.sol").read_bytes()).hexdigest() != UPSTREAM_SHA256:
    raise SystemExit("Pinned upstream Multicall3 source changed; review provenance first")
with tempfile.TemporaryDirectory(prefix="merkle-multicall-fixtures-") as temporary:
    project = Path(temporary)
    (project / "src").mkdir()
    for name in ("Multicall3", "FixtureMerklDistributor"):
        (project / f"src/{name}.sol").write_bytes((HERE / f"{name}.sol").read_bytes())
    (project / "foundry.toml").write_text(
        '[profile.default]\nsolc = "0.8.12"\nevm_version = "london"\n'
        'optimizer = true\noptimizer_runs = 200\nbytecode_hash = "none"\n'
    )
    subprocess.run(["forge", "build", "--offline", "--root", temporary], check=True)
    for name in ("Multicall3", "FixtureMerklDistributor"):
        artifact = json.loads((project / f"out/{name}.sol/{name}.json").read_text())
        document = {
            "abi": artifact["abi"],
            "bytecode": artifact["bytecode"]["object"],
            "source": f"{name}.sol",
            "sourceSha256": hashlib.sha256((HERE / f"{name}.sol").read_bytes()).hexdigest(),
            "compiler": artifact["metadata"]["compiler"]["version"],
        }
        (HERE / f"{name}.json").write_text(json.dumps(document, indent=2) + "\n")
        if name == "Multicall3":
            subset = [entry for entry in artifact["abi"] if entry.get("name") == "aggregate3"]
            if len(subset) != 1:
                raise SystemExit("Expected exactly one aggregate3 ABI entry")
            (SDK / "abi/Multicall3.json").write_text(json.dumps(subset, indent=2) + "\n")
