#!/usr/bin/env python3
"""Generate independent packed-recipe vectors using Foundry cast, not the Go codec."""
import json
from pathlib import Path
import subprocess


def cast(*args):
    return subprocess.run(["cast", *args], check=True, capture_output=True, text=True).stdout.strip()


def array(values):
    return "[" + ",".join(map(str, values)) + "]"


def step(operation, signature, *args):
    return f"0x{operation:02x}" + cast("abi-encode", signature, *args)[2:]


target = "0x1111111111111111111111111111111111111111"
input0 = 1 << 216
result0 = 2 << 216
result1 = result0 | 1
query = cast("abi-encode", "query(address,bytes,uint256[],uint256)", target,
             cast("calldata", "balanceOf(address)", "0x0000000000000000000000000000000000000000"),
             array([(4 << 224) | input0]), "0")
read = step(0, "read(uint256)", "0")
add = step(1, "add(uint256,uint256)", str(result0), "5")
call = step(11, "call(address,bytes,uint256[])", target,
            cast("calldata", "touch(uint256)", "0"), array([(4 << 224) | result1]))
queries, steps = array([query]), array([read, add, call])
recipe = cast("abi-encode", "recipe(bytes[],bytes[])", queries, steps)
document = {
    "generator": "Foundry cast; internal/recipes/testdata/generate.py",
    "query": query, "read": read, "add": add, "call": call,
    "recipe": recipe,
    "recipeHash": cast("keccak", recipe),
    "setRecipe": cast("calldata", "setRecipe(bytes[],bytes[])", queries, steps),
    "setRecipeTopic": cast("keccak", "SetRecipe(bytes32,bytes[],bytes[])"),
    "execute": cast("calldata", "execute(bytes[],bytes[],uint256[],bytes[])", queries, steps, "[17]", "[]"),
}
Path(__file__).with_name("core-abi.json").write_text(json.dumps(document, indent=2) + "\n")
