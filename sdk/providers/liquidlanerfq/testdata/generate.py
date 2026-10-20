#!/usr/bin/env python3
"""Reproduce local-only RFQ consumer fixtures using cached solc 0.8.37."""
import hashlib,json,subprocess,tempfile
from pathlib import Path
HERE=Path(__file__).resolve().parent
with tempfile.TemporaryDirectory(prefix="merkle-rfq-fixture-") as tmp:
 p=Path(tmp);(p/"src").mkdir();(p/"src/Fixture.sol").write_bytes((HERE/"Fixture.sol").read_bytes())
 (p/"foundry.toml").write_text('[profile.default]\nsolc = "0.8.37"\nevm_version = "osaka"\noptimizer = true\noptimizer_runs = 200\nbytecode_hash = "none"\n')
 subprocess.run(["forge","build","--offline","--root",tmp],check=True)
 for name in ("FixtureRFQAccount","FixtureRFQToken","FixtureRFQReactor"):
  a=json.loads((p/f"out/Fixture.sol/{name}.json").read_text())
  result={"abi":a["abi"],"bytecode":a["bytecode"]["object"],"compiler":a["metadata"]["compiler"]["version"],"source":"Fixture.sol","sourceSha256":hashlib.sha256((HERE/"Fixture.sol").read_bytes()).hexdigest()}
  (HERE/f"{name}.json").write_text(json.dumps(result,indent=2)+"\n")
