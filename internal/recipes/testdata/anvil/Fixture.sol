// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;

// Local integration fixture only. Administrative methods intentionally have no
// access controls; no fixture contract is suitable for deployment outside tests.
contract FixtureAccount {
    struct Action { address target; uint96 value; bytes data; bytes dataMask; bytes32[] proof; }
    bytes32 public merkleRoot;
    mapping(bytes32 => mapping(address => bool)) public hasRole;

    function configure(bytes32 root, bytes32[] calldata leaves, address executor) external {
        merkleRoot = root;
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
            require(computed == merkleRoot);
            (bool success, bytes memory result) = action.target.call(action.data);
            require(success);
            results[i] = result;
        }
    }
}

contract FixtureToken {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    address public lastCaller;
    uint256 public marker;

    function mint(address account, uint256 amount) external { balanceOf[account] += amount; }
    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        return true;
    }
    function approve(address spender, uint256 amount) external returns (bool) {
        require(amount == 0 || allowance[msg.sender][spender] == 0);
        allowance[msg.sender][spender] = amount;
        return true;
    }
    function touch(uint256 value) external { lastCaller = msg.sender; marker = value; }
}
