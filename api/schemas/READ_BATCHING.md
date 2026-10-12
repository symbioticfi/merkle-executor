# Read aggregation source provenance

Read-only source inspection on 2026-10-12 established the getter scopes below. No live transactions were used. These are the supported implementation assumptions when `chain.multicall3` is enabled. Operators must verify that configured deployments, custom targets and proxy upgrades retain the reviewed behavior. Source discovery is never performed automatically at runtime. ABI view annotations alone do not establish safety.

Sourcify source URL template: `https://sourcify.dev/server/v2/contract/1/<implementation>?fields=sources`. Proxy mapping metadata used `?fields=proxyResolution`; it establishes mappings at retrieval, not code verification at a runtime pinned block.

## Core and Liquid Lane

Pinned [adapters revision](https://github.com/symbioticfi/adapters/tree/bd5e235899de5e175a0001a913d47596f6f4f801): `bd5e235899de5e175a0001a913d47596f6f4f801`. Hashes below are SHA-256 of source bytes.

| Path relative to src/contracts/merkle-adapter | SHA256 | Safe getters |
|---|---|---|
| connectors/LiquidLaneConnector.sol | 54f34c9bca2a3005a3588f21d4cb0be90371ae8563f9bf929c9a8bb2e753e079 | account, outputAsset, signer, isUsedNonce, minDiscount, getAmountOut, remainingAssets |
| libraries/AssetRateLimits.sol | 3a8419c654e666956026d774e0b0c63fb308b244633f64897d72600acd707558 | remainingAssets downstream rate calculation |
| MerkleAccount.sol | 83e8cd39f5d56ca83b4618f6f289499f0b7289ec8e9b93e90ce890d20402cb55 | root, signatures; inherited hasRole |
| automations/RecipeExecutor.sol | 3f7eacac6e0c8794a3d7d0012c12b0c7ca0c6a9d1e70e264ea6ca202455ff639 | account, caller, recipeBlock, recipeHash |

LL inherited EIP712/Ownable: OZ submodule `cab19933c33c2ad1d4c7a84864a3601dddfd16f3`, `contracts/utils/cryptography/EIP712.sol` SHA256 `1023ca2a7f9590767a9cf7b7ed1259801d81c576787b7e25d5a40683c8c3fd97`, `contracts/access/Ownable.sol` SHA256 `38578bd71c0a909840e67202db527cc6b4e6b437e0f39f0c909da32c1e30cb81`. `eip712Domain` and `owner` are caller-independent. LL oracle/token downstream calls retain connector as sender in either mode.

Account inherited hasRole: OZ-upgradeable `14f52c54d3a1eefbda3d4071efba24d3c1e07e8a`, `contracts/access/AccessControlUpgradeable.sol` SHA256 `298e3937f41f3b27fd024741e9da72402b332d9d8ebe1575f94f1096ff66d07b`; explicit role/account map. `OwnableUpgradeable.sol` SHA256 `d20bb6bb32dd1fdb895914012ad08dc1e4bbd66df2e75a02c1ca97d884df70ed`.

## CoW

Settlement `0x9008D19f58AAbD9eD0D60971565AA8510560ab41`: `src/contracts/GPv2Settlement.sol` SHA256 `2878f0c7cf57b91a5ee6170eac15045c8c415993e676e2a23ac792a07607da63`, public filledAmount map; `src/contracts/mixins/GPv2Signing.sol` SHA256 `dc678895f9eb4cef3910f88621d99496da0daa5d14866e23a16b7cbb4e847a19`, immutable domainSeparator. All three Advance getters including pinned account.signatures safe. Persisted identity checks and terminal/idle precedence remain mandatory.

## Midas

All seven catalog ramp implementations share `contracts/abstract/ManageableVault.sol` SHA256 `ffc0a4a7982cdb096759a7884c791ef30214a8ffff015bb30f20c7bd9f9f845b` (public mToken/tokensConfig/minAmount) and `@openzeppelin/contracts-upgradeable/security/PausableUpgradeable.sol` SHA256 `c05b019a0b3bee8f3fac2da7c929f7d665b97d6d046aa35126615fff11205119` (paused storage). Four getters safe. Mappings:

| Asset | Ramp | Implementation |
|---|---|---|
| mF-ONE | 0x44b0440e35c596e858cEA433D0d82F5a985fD19C | 0x2118d1D95F82B0778c9335eAfB3a65838c4d8067 |
| mROX | 0xc33dAdA688f224c514682Ec6Ba940888d43C4b29 | 0x780D42A5A58e57318324d5666A6f638959aC2aA9 |
| mHYPER | 0x6Be2f55816efd0d91f52720f096006d63c366e98 | 0x570C15bC5FaF98531A8b351d69E22E41e3505E47 |
| CarryTradeUSDTRY | 0xD980df2A697bfd38279BE1Ee2bc13495c101d5C9 | 0x84a3A13eE09cF79e41E2EF6b926BC7b15f122882 |
| StockMarketTRBasis | 0x85A7A5FFf71EaEF79e76730F2E717A04aADea27B | 0x9E50EB61EcdcD2e8F559Def7CCAd1dE5A25f286B |
| mM1 | 0x70Ba3211f2584Bf1C8a2aCdF0a00dba559CE1Ffa | 0x326B2532bDc58aB4EC57CFe1495f9997f32b0C74 |
| mGLOBAL | 0x1e0fd66753198c7b8bA64edEe8d41D8628Bf20D7 | 0xE98a4Fb7a2e87AD888CceF0587DC820cf1a7CAbB |

## OpenEden

Express impl `0xcE7e7784c6741ace837721bdC44c19CD71d33F3b`: `contracts/extension/Express.sol` SHA256 `4189817c2ca7766fcecc051d15c2d9ca455900210077c6e964d313b35bee0f8e` token/redeemAsset public; `DepositRedeemLimiter.sol` SHA256 `3514fcfc6f27778591a6a7534eeb5ca585d3dc9de1edf69c370b03afd570183d` redeemMinimum public; `ExpressPausable.sol` SHA256 `8edece6fc101a3e450b5239c4cb7b1f8d0e43df6b970035566860bdd52fc86bf` pausedRedeem storage. All four safe.

## Noon

Handler `0x0DaBc0D9B270c9B0C4C77AaCeAa712b56D0F9178`: `contracts/WithdrawalHandler.sol` SHA256 `262670b0ba0c9f49bb81a80f0298eaf3d0009b5916ec44ee3d1c09ab20be62dc`; usn/withdrawPeriod/STAKING_VAULT_ROLE storage/constants, getWithdrawalRequest indexes explicit user/id. OZ `@openzeppelin/contracts/access/AccessControl.sol` SHA256 `1086a1ad3788972b885ff3f209da510615dde6214d46b29e1cd2a4924f66c06d`, hasRole indexes explicit account.

sUSN impl `0xebbcbc6672683e1956125e7c5e89e14ceac8cd3d`: inherited `@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC4626Upgradeable.sol` SHA256 `8c65f46c961932ab4113c4758fbc96aaf350e812b040a201603131be59ada737`; asset storage. All Noon stage getters safe; returned-role hasRole remains a dependent second request on request route.

## Pareto

Queue target `0x5cC24f44cCAa80DD2c079156753fc1e908F495DC` -> impl `0x492af7fd7759897db2BE8992e497Cce6cc10EB02`: `contracts/IdleCDOEpochQueue.sol` SHA256 `487911f25919caeda4dbfbf55c3568d5a462be7c81402b4be830474f63780534`; all consumed queue getters public storage/mappings.

CDO target `0x433D5B175148dA32Ffe1e1A37a939E1b7e79be4d` -> impl `0x67F839a44E8B6d2CD403d9cB14f3862F2b85E9f8`: `contracts/IdleCDOEpochVariant.sol` SHA256 `9707cd2a4fef1001f9438754a2dfa0fe1d2c4816c2460c7afee7665006ec6d59`, isEpochRunning/epochEndDate/allowInstantWithdraw/defaulted storage and isWalletAllowed explicit account; `contracts/IdleCDOStorage.sol` SHA256 `b242df1ecae064e9830eda35e89b295c8c7039433d7fd09ce2fb7b177af3c783`, token/strategy/AATranche storage. isWalletAllowed downstream call retains CDO as sender in both modes.

Strategy docs-pinned impl `0x62568889198F1bAb603E26dA7b6c1808838fE489`: `contracts/strategies/idle/IdleCreditVault.sol` SHA256 `e2ed8441a6716b0b9fb30e1ab3fbf39d1f66e021b70fc0e9d419954d54e5c6ce`; token/idleCDO/epochNumber and all consumed withdraw/APR0/request getters are public storage/mappings. All Pareto first-stage getters safe; conditional apr0RateByEpoch argument remains dependent.

## Figure

PRIME target `0x19ebb35279A16207Ec4ba82799CC64715065F7F6` -> impl `0x881fe0e5e91C54FaBBf0198A1bB2fC6E5747D4C5`; wYLDS target `0x6aD038cA6C04e885630851278ca0a856Ad9a66Cc` -> impl `0x06e0b9155A3cf07F41aC826CCFEE7ef8413A9723`. Both inherit `@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC4626Upgradeable.sol` SHA256 `8769acf7e5cb0c978bdf117255097c59afb250b725f9114145eed6e59a9f35f0`, asset/previewRedeem. PRIME `contracts/StakingVault.sol` SHA256 `81fccd7e2b2f9ead6ffccd6c676eb75089b02ab30de1340b6238fa808ce512fc`, conversion queries stored NAV oracle from PRIME in either mode. wYLDS `contracts/YieldVault.sol` SHA256 `d96fc4056882cca4ac8dc0b98146a38e68949d9d7d736cda31e3bd459e4fc366`, pendingRedemptions public. All four consumed Figure getters safe.

wYLDS top `contracts/YieldVaultV2.sol` SHA256 `f5cbe3fcad60f4c65e2ecf72bb9fb800a64cbd7f8a2e9d89ca4556360f11f5f3` adds no getter overrides. Noon top `contracts/StakingVaultOFTUpgradeableHyperlane.sol` SHA256 `360c5029f48c796fae765c85274872f1c9fbbbcc5e50c92ee942a0aaf20e4c8f` adds no asset override.

## Securitize partial verification

Modern impl `0x76ffdD3628A370a05Ea7271D1A0690BcC936779F`: `contracts/off-ramp/SecuritizeOffRamp.sol` SHA256 `d296224509b24bf437a8c0fbc4cd0992bcf1e581659ddd56714d8d4f551eb139`, assetAddress/liquidityProvider public, calculateLiquidityTokenAmount and availableLiquidity caller-independent; downstream dependencies are called from ramp in either mode. OZ-upgradeable `utils/PausableUpgradeable.sol` SHA256 `78df1c8d83d858b123485f7cc2e15e40e28d787a21f949374ee34aa45c1ad69c`, paused storage. Ramp-local getters safe.

Legacy USDC `0x31D3F59Ad4aAC0eeE2247c65EBE8Bf6E9E470a53`: `contracts/Redemption.sol` SHA256 `1f236f389de6fc0cd2446eeeb389e21972cd18497a567b4e0dad0413a57355ef`. USDtb `0x4580D3c5fFF079BDe60Af9e98B62d4FB031eE0a0` same path SHA256 `c14af4566c672d9512cf7574ce82663d9f8d185cdab2075dcd2f0af38dfc7de0`. asset/liquidity/settlement public, inherited OZ `security/Pausable.sol` SHA256 `2072248d2f79e661c149fd6a6593a8a3f038466557c9b75e50e0b001bcb5cf97`. Ramp-local getters safe.

Residual modern discovered liquidityProvider.liquidityToken and legacy discovered settlement.availableLiquidity are interface-only in these sources and target addresses mutable/unpinned locally. Preserve direct caller via isolated JSON in same independent batch; do not mark safe solely by selector.

VBILL mainnet token `0x2255718832bC9fD3bE1CaF75084F4803DA14FF01` -> DSToken impl `0xaf88485377cCa5e690e7DcB1BA81370F0B575156`: `contracts/data-stores/TokenDataStore.sol` SHA256 `3a4d8fc24e268ce7fc3a2011dbaf3a15f5f0695f0c18f751661261073ff53c98`, public decimals; `contracts/token/DSToken.sol` SHA256 `e7dea9bb5b3cad88d2eea7d8d37a56e958aafd5145c859533182f3b9672d0959`. ARKVX mainnet token `0xDf1c8e71CbDF48af50B36F96AD2eb6F5094Ba72a` -> docs-pinned impl `0xD57499B239700DDB99bb5B85596DDAb3745bC24d`: TokenDataStore.sol SHA256 `ada1547524632ce0714c108b9e9a181af84f5a0b4b0903447bd7503d81aba6d7`, DSToken.sol SHA256 `195afc15df5d4d55a9b4ee289585c031d8237e00827abce35d9f9c71e4452780`, public decimals. Do not generalize these sources to all 26 token deployments. BUIDL implementation was not sufficiently resolvable, so its decimals remains an isolated JSON call.

Legacy payment decimals verified: USDC target `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48` -> impl `0x43506849D7C04F9138D1A2050bbF3A0c054402dd`; metadata path ending `contracts/v1/FiatTokenV1.sol` SHA256 `60cf9152586da7464f9f2a9aec2e67151edce2ec0eafad9061111c7ddecbd253`, public decimals. The source metadata contains an upstream developer's absolute filename; it is not a local file and should be described by its contract suffix. USDtb target `0xC139190F447e929f090Edeb554D95AbB8b18aC1C` -> impl `0x9D6d77a21702b9AFcF924983fbFB84AAAAe79589`; `lib/openzeppelin-contracts-upgradeable-v4/contracts/token/ERC20/ERC20Upgradeable.sol` SHA256 `41b11f70764d014fd808bcaa33aa16dbad5554263c4e7e9ef1f3359ada116a11`, returns18 and no implementation override found. These two explicit catalog payment-token deployments may safely aggregate decimals.

## Centrifuge partial verification

Known share vault(asset) getters are public maps: JAAA `0x5a0F93D040De44e78F251b03c43be9CF317Dcf64`, JTRSY `0x8c213ee79581Ff4984583C6a801e5263418C4b86`: `src/token/Tranche.sol` SHA256 `ed1cba94b2f44a95ce2761100389ebddcf3c1a364a13f5bdd5b30d75a22d1018`; deJAAA `0xAAA0008C8CF3A7Dca931adaF04336A5D808C82Cc`: `src/spoke/ShareToken.sol` SHA256 `f6454db3e34932c6f7702fcfd1f2c34e7c9e56e6be4d533f1b4915c99170ecf7`. Source URLs use the share address in the same Sourcify template. HYB/deJTRSY share sources and vault asset/maxRedeem implementations were not sufficiently accessible; retain isolated JSON for those calls. Centrifuge stages currently remain JSON arrays: the one verified share getter alone has no aggregation benefit, and the remaining getters must execute in isolation.

## Exclusions

Arbitrary configured data RPC reads (`From=snapshot.Account`), arbitrary ERC1271 signatures, full execution simulations and unknown external getter implementations retain original isolated eth_call context. JSON batching still eliminates independent network round trips. All final canonical fences and hash-linked ancestry walks remain ordered singles; writes/signing/nonce admission are excluded.
