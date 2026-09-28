#!/usr/bin/env python3
"""Build local async issuer fixtures without any external chain or deployment."""
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile

HERE = Path(__file__).resolve().parent
source = HERE / "Fixture.sol"
with tempfile.TemporaryDirectory(prefix="merkle-issuer-fixtures-") as temporary:
    project = Path(temporary)
    (project / "src").mkdir()
    (project / "src/Fixture.sol").write_bytes(source.read_bytes())
    (project / "foundry.toml").write_text(
        '[profile.default]\nsolc = "0.8.37"\nevm_version = "osaka"\n'
        'optimizer = true\noptimizer_runs = 200\nbytecode_hash = "none"\n'
    )
    subprocess.run(["forge", "build", "--offline", "--root", temporary], check=True)
    for contract in ("FixtureShare", "FixtureAsyncIssuer"):
        artifact = json.loads((project / f"out/Fixture.sol/{contract}.json").read_text())
        document = {
            "abi": artifact["abi"],
            "bytecode": artifact["bytecode"]["object"],
            "source": "Fixture.sol",
            "sourceSha256": hashlib.sha256(source.read_bytes()).hexdigest(),
            "compiler": artifact["metadata"]["compiler"]["version"],
        }
        (HERE / f"{contract}.json").write_text(json.dumps(document, indent=2) + "\n")
