// SPDX-License-Identifier: MIT
pragma solidity 0.8.12;

// Local-only getter fixture. Requiring the aggregate caller catches accidental
// direct reads while argument-keyed mappings exercise per-user/token batching.
contract FixtureMerklDistributor {
    struct Claim { uint208 amount; uint48 timestamp; bytes32 merkleRoot; }
    bytes32 private root;
    address private immutable aggregator;
    bool public fail;
    mapping(address => mapping(address => Claim)) private claims;
    mapping(address => mapping(address => address)) private recipients;

    constructor(address multicall) { aggregator = multicall; }
    modifier aggregated() { require(msg.sender == aggregator && !fail, "fixture getter unavailable"); _; }
    function configure(bytes32 newRoot, address user, address token, uint208 amount, address recipient) external {
        root = newRoot;
        claims[user][token] = Claim(amount, uint48(block.timestamp), newRoot);
        recipients[user][token] = recipient;
    }
    function setRecipient(address user, address token, address recipient) external { recipients[user][token] = recipient; }
    function setFailure(bool enabled) external { fail = enabled; }
    function getMerkleRoot() external view aggregated returns (bytes32) { return root; }
    function claimed(address user, address token) external view aggregated returns (uint208, uint48, bytes32) {
        Claim memory value = claims[user][token];
        return (value.amount, value.timestamp, value.merkleRoot);
    }
    function claimRecipient(address user, address token) external view aggregated returns (address) { return recipients[user][token]; }
}
