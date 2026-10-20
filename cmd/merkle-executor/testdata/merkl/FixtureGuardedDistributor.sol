// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;

interface FixtureMint { function mint(address,uint256) external; }
// Local stateful distributor fixture: proof, caller, cumulative ledger and payout
// are enforced independently from the account's source-pinned capability guards.
contract FixtureGuardedDistributor {
    struct Claim { uint208 amount; uint48 timestamp; bytes32 merkleRoot; }
    bytes32 public root;
    mapping(address => mapping(address => Claim)) public claimed;
    mapping(address => mapping(address => address)) public claimRecipient;
    uint256 public payoutBps = 10000;
    function setRoot(bytes32 value) external { root = value; }
    function setClaimed(address user,address token,uint208 amount) external { claimed[user][token] = Claim(amount,uint48(block.timestamp),root); }
    function setRecipient(address user,address token,address recipient) external { claimRecipient[user][token] = recipient; }
    function setPayoutBps(uint256 value) external { payoutBps = value; }
    function getMerkleRoot() external view returns (bytes32) { return root; }
    function claim(address[] calldata users,address[] calldata tokens,uint256[] calldata amounts,bytes32[][] calldata proofs) external {
        for (uint256 i;i<users.length;++i) {
            address recipient=claimRecipient[users[i]][tokens[i]];
            if (recipient==address(0)) recipient=claimRecipient[users[i]][address(0)];
            if (recipient==address(0)) recipient=users[i];
            _claim(users[i],tokens[i],amounts[i],proofs[i],recipient);
        }
    }
    function claimWithRecipient(address[] calldata users,address[] calldata tokens,uint256[] calldata amounts,bytes32[][] calldata proofs,address[] calldata recipients,bytes[] calldata datas) external {
        for (uint256 i;i<users.length;++i) { require(datas[i].length==0); _claim(users[i],tokens[i],amounts[i],proofs[i],recipients[i]); }
    }
    function _claim(address user,address token,uint256 amount,bytes32[] calldata proof,address recipient) private {
        require(msg.sender==user && amount<=type(uint208).max);
        bytes32 hash=keccak256(abi.encode(user,token,amount));
        for (uint256 i;i<proof.length;++i) { bytes32 other=proof[i]; hash=hash<other?keccak256(abi.encodePacked(hash,other)):keccak256(abi.encodePacked(other,hash)); }
        require(hash==root);
        uint256 delta=amount-claimed[user][token].amount;
        claimed[user][token]=Claim(uint208(amount),uint48(block.timestamp),root);
        FixtureMint(token).mint(recipient,delta*payoutBps/10000);
    }
}
