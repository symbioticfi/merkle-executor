// SPDX-License-Identifier: MIT
pragma solidity 0.8.38;

/// @dev eth_call-only adapter. Executes the unchanged runtime at the original
/// address/storage/caller and returns the values omitted by Multicallable.
contract BatchSimulation {
    bytes32 private constant IMPLEMENTATION = keccak256("merkle-executor.batch-simulation");

    struct ExecutorCall {
        address executor;
        bytes data;
    }

    /// @dev Installed at the delegated caller only during eth_call.
    function simulateCalls(ExecutorCall[] calldata calls) external returns (bytes[] memory) {
        require(msg.sender == address(this), "caller must be self");
        return callAll(calls);
    }

    /// @dev Reads back into the AccessManager use its original runtime and storage.
    function simulateForwardedCalls(bytes calldata runtime, ExecutorCall[] calldata calls) external returns (bytes[] memory) {
        install(runtime);
        return callAll(calls);
    }

    function callAll(ExecutorCall[] calldata calls) private returns (bytes[] memory results) {
        results = new bytes[](calls.length);
        for (uint256 i; i < calls.length; ++i) {
            bool success;
            (success, results[i]) = calls[i].executor.call(calls[i].data);
            if (!success) {
                bytes memory reason = results[i];
                assembly { revert(add(reason, 32), mload(reason)) }
            }
        }
    }

    function install(bytes calldata runtime) private returns (address implementation) {
        // Constructor returning the supplied runtime verbatim.
        bytes memory creation = abi.encodePacked(hex"61", uint16(runtime.length), hex"80600a3d393df3", runtime);
        assembly { implementation := create(0, add(creation, 32), mload(creation)) }
        require(implementation != address(0), "runtime deployment failed");
        bytes32 slot = IMPLEMENTATION;
        assembly { tstore(slot, implementation) }
    }

    function simulate(bytes calldata runtime, bytes[] calldata calls) external returns (bytes[] memory results) {
        address implementation = install(runtime);
        results = new bytes[](calls.length);
        for (uint256 i; i < calls.length; ++i) {
            bool success;
            (success, results[i]) = implementation.delegatecall(calls[i]);
            if (!success) {
                bytes memory reason = results[i];
                assembly { revert(add(reason, 32), mload(reason)) }
            }
        }
    }

    fallback() external payable {
        bytes32 slot = IMPLEMENTATION;
        assembly {
            let implementation := tload(slot)
            if iszero(implementation) { revert(0, 0) }
            calldatacopy(0, 0, calldatasize())
            let success := delegatecall(gas(), implementation, 0, calldatasize(), 0, 0)
            returndatacopy(0, 0, returndatasize())
            if iszero(success) { revert(0, returndatasize()) }
            return(0, returndatasize())
        }
    }
}
