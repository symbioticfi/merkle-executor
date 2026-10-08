#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
ABIGEN=${ABIGEN:-abigen}
case "$("$ABIGEN" --version)" in
    "abigen version 1.17.5"*) ;;
    *) echo "abigen 1.17.5 is required" >&2; exit 1 ;;
esac
for spec in RecipeExecutor:recipeexecutor MerkleAccount:merkleaccount ERC20:erc20; do
    contract=${spec%:*}
    package=${spec#*:}
    mkdir -p "sdk/bindings/$package"
    "$ABIGEN" --v2 --abi "sdk/abi/$contract.json" --pkg "$package" --type "$contract" --out "sdk/bindings/$package/$contract.go"
done
