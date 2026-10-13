// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;
// Local EVM fixtures; never deployed outside ephemeral Anvil.
contract LensProbe {
 address public immutable executor; address public immutable caller;
 constructor(address executor_,address caller_) { executor=executor_; caller=caller_; }
 function contextValue() external view returns(uint256) { require(msg.sender==executor && tx.origin==caller); return 42; }
 function preview(uint256 amount) external view returns(uint256) { require(msg.sender==executor && tx.origin==caller); return amount*2; }
 function callerCodeSize() external view returns(uint256) { return msg.sender.code.length; }
}
contract ZeroApprovalToken {
 function allowance(address,address) external pure returns(uint256) { revert("must not read allowance for zero"); }
 function approve(address,uint256 amount) external pure returns(bool) { require(amount==0); return true; }
}
interface IRewardToken { function mint(address,uint256) external; }
contract FixtureAutomaticRewards {
 bytes32 public getMerkleRoot;
 mapping(address=>mapping(address=>uint208)) private amounts;
 function setRoot(bytes32 root) external { getMerkleRoot=root; }
 function claimed(address user,address token) external view returns(uint208,uint48,bytes32) { return (amounts[user][token],0,getMerkleRoot); }
 function claimRecipient(address,address) external pure returns(address) { return address(0); }
 function claim(address[] calldata users,address[] calldata tokens,uint256[] calldata totals,bytes32[][] calldata proofs) external {
  require(users.length==tokens.length && users.length==totals.length && users.length==proofs.length);
  for(uint256 i;i<users.length;++i) {
   require(users[i]==msg.sender && totals[i]<=type(uint208).max);
   bytes32 root=keccak256(abi.encode(users[i],tokens[i],totals[i]));
   for(uint256 j;j<proofs[i].length;++j) root=root<proofs[i][j] ? keccak256(abi.encodePacked(root,proofs[i][j])) : keccak256(abi.encodePacked(proofs[i][j],root));
   require(root==getMerkleRoot);uint256 delta=totals[i]-amounts[users[i]][tokens[i]];amounts[users[i]][tokens[i]]=uint208(totals[i]);IRewardToken(tokens[i]).mint(users[i],delta);
  }
 }
}
