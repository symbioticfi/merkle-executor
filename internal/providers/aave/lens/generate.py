#!/usr/bin/env python3
"""Compile the eth_call-only Aave probe with its shared executor-prefix source."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
HERE=Path(__file__).resolve().parent
ROOT=HERE.parents[3]
SOURCES={"FixtureAave.sol":HERE/"FixtureAave.sol", "AaveBorrowProbe.sol":HERE/"AaveBorrowProbe.sol", "RecipePrefixLens.sol":ROOT/"sdk/recipes/lens/RecipePrefixLens.sol"}
parser=argparse.ArgumentParser()
parser.add_argument("--check",action="store_true")
args=parser.parse_args()
with tempfile.TemporaryDirectory(prefix="aave-borrow-probe-") as directory:
    root=Path(directory)
    (root/"src").mkdir()
    for name,path in SOURCES.items(): (root/"src"/name).write_bytes(path.read_bytes())
    (root/"foundry.toml").write_text('[profile.default]\nsolc="0.8.37"\nevm_version="cancun"\noptimizer=true\noptimizer_runs=200\nvia_ir=true\nbytecode_hash="none"\n')
    subprocess.run(["forge","build","--root",directory],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
    artifact=json.loads((root/"out/AaveBorrowProbe.sol/AaveBorrowProbe.json").read_text())
    fixture=json.loads((root/"out/FixtureAave.sol/FixtureAave.json").read_text())
    fixture_text=json.dumps({"abi":fixture["abi"],"bytecode":fixture["bytecode"]["object"],"sourceSHA256":hashlib.sha256((HERE/"FixtureAave.sol").read_bytes()).hexdigest()},indent=2)+"\n"
    fixture_path=HERE/"FixtureAave.json"
    if args.check:
        if fixture_path.read_text()!=fixture_text:raise SystemExit("stale Aave fixture artifact")
    else:fixture_path.write_text(fixture_text)
    result={"compiler":artifact["metadata"]["compiler"]["version"],"evmVersion":"cancun","sources":{n:hashlib.sha256(p.read_bytes()).hexdigest() for n,p in SOURCES.items()},"runtime":artifact["deployedBytecode"]["object"]}
    text=json.dumps(result,indent=2)+"\n"
    path=HERE/"AaveBorrowProbe.json"
    if args.check:
        if path.read_text()!=text:raise SystemExit("stale Aave probe artifact")
    else:path.write_text(text)
