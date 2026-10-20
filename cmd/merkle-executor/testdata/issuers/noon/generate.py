#!/usr/bin/env python3
"""Compile the pinned verified Noon source closure with native solc 0.8.28."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess

HERE = Path(__file__).resolve().parent
parser = argparse.ArgumentParser()
parser.add_argument("--update", action="store_true", help="write generated artifacts")
parser.add_argument("--check", action="store_true", help="verify artifacts without writing")
args = parser.parse_args()
if args.update and args.check:
    parser.error("choose --update or --check")
provenance = json.loads((HERE / "provenance.json").read_text())
sources = json.loads((HERE / "sources.json").read_text())
for name, expected in provenance["sources"].items():
    actual = hashlib.sha256(sources[name]["content"].encode()).hexdigest()
    if actual != expected:
        raise SystemExit(f"Noon source hash changed: {name}")
if set(sources) != set(provenance["sources"]):
    raise SystemExit("Noon source closure changed")
fixture = (HERE / "Fixture.sol").read_text()
sources["Fixture.sol"] = {"content": fixture}
solc = os.environ.get("NOON_SOLC_BIN") or shutil.which("solc")
if not solc:
    for location in (Path.home() / "Library/Application Support/svm/0.8.28/solc-0.8.28", Path.home() / ".svm/0.8.28/solc-0.8.28"):
        if location.is_file():
            solc = str(location)
            break
if not solc:
    raise SystemExit("native solc 0.8.28 required; set NOON_SOLC_BIN")
version = subprocess.check_output([solc, "--version"], text=True)
if "0.8.28+commit.7893614a" not in version:
    raise SystemExit("Noon fixture requires exact solc 0.8.28+commit.7893614a")
settings = dict(provenance["settings"])
settings["outputSelection"] = {"*": {"*": ["abi", "evm.bytecode.object", "storageLayout"]}}
request = {"language": "Solidity", "sources": sources, "settings": settings}
compiled = json.loads(subprocess.check_output([solc, "--standard-json"], input=json.dumps(request).encode()))
errors = [e["formattedMessage"] for e in compiled.get("errors", []) if e["severity"] == "error"]
if errors:
    raise SystemExit("\n".join(errors))
artifact_hashes = {}
for filename, name in (("contracts/RedeemHandlerV2.sol", "RedeemHandlerV2"), ("Fixture.sol", "FixtureNoonToken"), ("Fixture.sol", "FixtureNoonOracle")):
    contract = compiled["contracts"][filename][name]
    document = {"abi": contract["abi"], "bytecode": "0x" + contract["evm"]["bytecode"]["object"],
                "storageLayout": contract["storageLayout"], "compiler": provenance["compiler"],
                "settings": provenance["settings"], "sourceSha256": provenance["sources"].get(filename, hashlib.sha256(fixture.encode()).hexdigest())}
    encoded = json.dumps(document, indent=2) + "\n"
    target = HERE / f"{name}.json"
    artifact_hashes[target.name] = hashlib.sha256(encoded.encode()).hexdigest()
    if args.update:
        target.write_text(encoded)
    elif not target.is_file() or target.read_text() != encoded:
        raise SystemExit(f"Noon artifact is stale: {target.name}; regenerate with --update")
if args.update:
    provenance["artifacts"] = artifact_hashes
    provenance["fixtureSha256"] = hashlib.sha256(fixture.encode()).hexdigest()
    (HERE / "provenance.json").write_text(json.dumps(provenance, indent=2) + "\n")
elif provenance.get("artifacts") != artifact_hashes or provenance.get("fixtureSha256") != hashlib.sha256(fixture.encode()).hexdigest():
    raise SystemExit("Noon artifact provenance is stale; regenerate with --update")
print("Noon source-pinned artifacts " + ("generated" if args.update else "verified"))
