// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;

// Local integration fixture only. Administrative methods intentionally have no
// access controls; no fixture contract is suitable for deployment outside tests.
contract FixtureGuardedAccount {
    struct Action { address target; uint96 value; bytes data; bytes dataMask; bytes32[] proof; }
    event Manage(Action[] actions);
    bytes32 public root;
    address public capability;
    function setCapability(address value) external { capability = value; }
    fallback() external {
        address target = capability;
        assembly {
            calldatacopy(0, 0, calldatasize())
            let ok := delegatecall(gas(), target, 0, calldatasize(), 0, 0)
            returndatacopy(0, 0, returndatasize())
            if iszero(ok) { revert(0, returndatasize()) }
            return(0, returndatasize())
        }
    }
    function adapter() external view returns (address) { return address(this); }
    mapping(bytes32 => mapping(address => bool)) public hasRole;

    function configure(bytes32 newRoot, bytes32[] calldata leaves, address executor) external {
        root = newRoot;
        for (uint256 i; i < leaves.length; ++i) hasRole[leaves[i]][executor] = true;
    }

    function manage(Action[] calldata actions) external returns (bytes[] memory results) {
        results = new bytes[](actions.length);
        for (uint256 i; i < actions.length; ++i) {
            Action calldata action = actions[i];
            require(action.value == 0 && action.data.length == action.dataMask.length);
            bytes memory masked = new bytes(action.data.length);
            for (uint256 j; j < masked.length; ++j) masked[j] = action.data[j] & ~action.dataMask[j];
            bytes32 leaf = keccak256(bytes.concat(keccak256(abi.encode(
                action.target, bytes1(0), keccak256(action.dataMask), keccak256(masked)
            ))));
            require(hasRole[leaf][msg.sender]);
            bytes32 computed = leaf;
            for (uint256 j; j < action.proof.length; ++j) {
                bytes32 other = action.proof[j];
                computed = computed < other ? keccak256(abi.encodePacked(computed, other)) : keccak256(abi.encodePacked(other, computed));
            }
            require(computed == root);
            (bool success, bytes memory result) = action.target.call(action.data);
            require(success);
            results[i] = result;
        }
        emit Manage(actions);
    }
}
