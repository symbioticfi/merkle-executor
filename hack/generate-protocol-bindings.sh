#!/usr/bin/env sh
set -eu
cd "$(dirname "$0")/.."
ABIGEN=${ABIGEN:-abigen}
case "$("$ABIGEN" --version)" in
    "abigen version 1.17.5"*) ;;
    *) echo "abigen 1.17.5 is required" >&2; exit 1 ;;
esac
for spec in LiquidConnector:liquidconnector MerklDistributor:merkldistributor Multicall3:multicall3 CowSettlement:cowsettlement \
    MidasRedemption:midasredemption CentrifugeShare:centrifugeshare AsyncRedeemVault:asyncredeemvault \
    FigureYieldVault:figureyieldvault OpenEdenExpress:openedenexpress ParetoQueue:paretoqueue \
    ParetoCDO:paretocdo ParetoStrategy:paretostrategy ERC4626:erc4626 \
    NoonWithdrawalHandler:noonwithdrawalhandler NoonRedeemHandlerV2:noonredeemhandlerv2 \
    SecuritizeOffRamp:securitizeofframp SecuritizeLegacyOffRamp:securitizelegacyofframp \
    SecuritizeToken:securitizetoken SecuritizeTopology:securitizetopology; do
    contract=${spec%:*}
    package=${spec#*:}
    mkdir -p "api/bindings/$package"
    "$ABIGEN" --v2 --abi "api/abi/$contract.json" --pkg "$package" --type "$contract" --out "api/bindings/$package/$contract.go"
done
