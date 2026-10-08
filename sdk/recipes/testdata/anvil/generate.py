#!/usr/bin/env python3
"""Rebuild local integration fixtures; requires Foundry and current adapters/out."""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
ADAPTERS = ROOT.parent / "adapters"

def artifact(source, path, output):
    document = json.loads(path.read_text())
    result = {"abi": document["abi"], "bytecode": document["bytecode"]["object"],
              "source": source, "compiler": document["metadata"]["compiler"]["version"]}
    (HERE / output).write_text(json.dumps(result, indent=2) + "\n")

with tempfile.TemporaryDirectory(prefix="merkle-keeper-fixtures-") as temporary:
    project = Path(temporary)
    (project / "src").mkdir()
    (project / "src/Fixture.sol").write_bytes((HERE / "Fixture.sol").read_bytes())
    (project / "foundry.toml").write_text('[profile.default]\nsolc = "0.8.37"\nevm_version = "osaka"\noptimizer = true\noptimizer_runs = 200\nbytecode_hash = "none"\n')
    subprocess.run(["forge", "build", "--offline", "--root", temporary], check=True)
    for contract in ("FixtureAccount", "FixtureToken"):
        artifact("Fixture.sol", project / f"out/Fixture.sol/{contract}.json", f"{contract}.json")

source = "src/contracts/merkle-adapter/automations/RecipeExecutor.sol"
executor_artifact = json.loads((ADAPTERS / "out/RecipeExecutor.sol/RecipeExecutor.json").read_text())
for name, metadata in executor_artifact["metadata"]["sources"].items():
    if not name.startswith("src/"):
        continue
    current = subprocess.run(["cast", "keccak", "0x" + (ADAPTERS / name).read_bytes().hex()],
                             check=True, capture_output=True, text=True).stdout.strip()
    if current != metadata["keccak256"]:
        raise SystemExit(f"Stale RecipeExecutor artifact: rebuild adapters ({name})")
artifact(source, ADAPTERS / "out/RecipeExecutor.sol/RecipeExecutor.json", "RecipeExecutor.json")
manifest = {}
for source in executor_artifact["metadata"]["sources"]:
    if source.startswith("src/"):
        manifest[source] = hashlib.sha256((ADAPTERS / source).read_bytes()).hexdigest()
manifest["Fixture.sol"] = hashlib.sha256((HERE / "Fixture.sol").read_bytes()).hexdigest()
(HERE / "sources-sha256.json").write_text(json.dumps(manifest, indent=2) + "\n")
