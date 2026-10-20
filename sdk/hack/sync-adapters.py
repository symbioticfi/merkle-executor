#!/usr/bin/env python3
"""Refresh vendored contract ABIs and integration fixtures from the pinned source."""
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
source = json.loads((ROOT / "abi/source.json").read_text())
checkout = os.environ.get("MERKLE_ADAPTERS_REPO")
if not checkout:
    raise SystemExit("Set MERKLE_ADAPTERS_REPO to the pinned adapters checkout")
adapters = Path(checkout).resolve()
revision = subprocess.check_output(["git", "-C", str(adapters), "rev-parse", "HEAD"], text=True).strip()
if revision != source["revision"]:
    raise SystemExit(f"Expected adapters {source['revision']}, got {revision}")
if subprocess.check_output(["git", "-C", str(adapters), "status", "--porcelain", "--untracked-files=all", "--", "src", "lib", "foundry.toml", "remappings.txt"], text=True):
    raise SystemExit("Adapters checkout has source or dependency changes")

digests = {}
for contract, destination in (
    ("RecipeExecutor", "abi/RecipeExecutor.json"),
    ("MerkleAccount", "abi/MerkleAccount.json"),
    ("IERC20", "abi/ERC20.json"),
):
    artifact = json.loads((adapters / f"out/{contract}.sol/{contract}.json").read_text())
    for name, metadata in artifact["metadata"]["sources"].items():
        if name not in digests:
            digests[name] = subprocess.check_output(["cast", "keccak", "0x" + (adapters / name).read_bytes().hex()], text=True).strip()
        if digests[name] != metadata["keccak256"]:
            raise SystemExit(f"Stale {contract} artifact: rebuild adapters ({name})")
    (ROOT / destination).write_text(json.dumps(artifact["abi"], indent=2) + "\n")

subprocess.run(["python3", str(ROOT / "recipes/testdata/anvil/generate.py")], check=True)
