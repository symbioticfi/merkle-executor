#!/usr/bin/env python3
"""Build local Merkl fixtures offline with the flattened pinned capability source."""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
HERE=Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix="merkl-guarded-fixtures-") as temporary:
    root=Path(temporary)
    (root/"src").mkdir()
    for path in HERE.glob("*.sol"):
        (root/"src"/path.name).write_bytes(path.read_bytes())
    (root/"foundry.toml").write_text('[profile.default]\nsolc="0.8.37"\nevm_version="osaka"\noptimizer=true\noptimizer_runs=200\nvia_ir=true\nbytecode_hash="none"\n[profile.default.lint]\nlint_on_build=false\n')
    subprocess.run(["forge","build","--offline","--root",temporary],check=True)
    for name in ("RecipeCapability","FixtureGuardedAccount","FixtureGuardedDistributor"):
        artifact=json.loads((root/f"out/{name}.sol/{name}.json").read_text())
        document={"abi":artifact["abi"],"bytecode":artifact["bytecode"]["object"],"source":f"{name}.sol","sourceSha256":hashlib.sha256((HERE/f"{name}.sol").read_bytes()).hexdigest(),"compiler":artifact["metadata"]["compiler"]["version"]}
        (HERE/f"{name}.json").write_text(json.dumps(document,indent=2)+"\n")
