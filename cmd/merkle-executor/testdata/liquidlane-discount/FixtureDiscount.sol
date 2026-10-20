// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;
import {LiquidLaneConnector,AssetRateLimits} from "./LiquidLaneConnector.sol";

// Administrative seed methods exist only in local integration fixtures.
contract FixtureDiscountConnector is LiquidLaneConnector {
    constructor() LiquidLaneConnector(address(0),address(0),address(0)) {}
    function seed(address account_,address asset,address token,address oracle,address signer_) external {
        account=account_;outputAsset=asset;oracles[token]=oracle;signer=signer_;
        assetLimits[token]=AssetRateLimits.AssetLimit(150,86400,uint48(block.timestamp),0);
    }
}
contract FixtureDiscountOracle {
    function getPrice() external pure returns(uint256){return 2e18;}
}
contract FixtureDiscountToken {
    uint8 public constant decimals=18;
    mapping(address=>uint256) public balanceOf;
    mapping(address=>mapping(address=>uint256)) public allowance;
    uint256 public payoutBps=10000;
    function setPayoutBps(uint256 value) external {payoutBps=value;}
    function mint(address to,uint256 amount) external {balanceOf[to]+=amount;}
    function approve(address spender,uint256 amount) external returns(bool){allowance[msg.sender][spender]=amount;return true;}
    function transfer(address to,uint256 amount) external returns(bool){balanceOf[msg.sender]-=amount;balanceOf[to]+=amount*payoutBps/10000;return true;}
    function transferFrom(address from,address to,uint256 amount) external returns(bool){allowance[from][msg.sender]-=amount;balanceOf[from]-=amount;balanceOf[to]+=amount;return true;}
}
contract FixtureDiscountFunding {
    address public asset;address public account;
    function seed(address asset_,address account_) external {asset=asset_;account=account_;}
    function releasedLiquidity() external pure returns(uint256){return 0;}
    function stagedLiquidity() external pure returns(uint256){return 0;}
    function pendingDeallocation() external pure returns(uint256){return 0;}
    function push(uint256 amount) external {FixtureDiscountToken(asset).transfer(account,amount);}
}
