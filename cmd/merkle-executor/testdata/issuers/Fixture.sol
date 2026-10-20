// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;

// Local lifecycle fixture only. Minting, vault registration and fulfillment are
// deliberately permissionless to keep issuer administration outside this test.
contract FixtureShare {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    mapping(address => address) public vault;

    function mint(address account, uint256 amount) external { balanceOf[account] += amount; }
    function setVault(address asset, address newVault) external { vault[asset] = newVault; }
    function approve(address spender, uint256 amount) external returns (bool) {
        require(amount == 0 || allowance[msg.sender][spender] == 0);
        allowance[msg.sender][spender] = amount;
        return true;
    }
    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        allowance[from][msg.sender] -= amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        return true;
    }
}

interface IFixtureOutput {
    function transfer(address to, uint256 amount) external returns (bool);
}

contract FixtureAsyncIssuer {
    address public immutable asset;
    address public immutable share;
    mapping(address => uint256) public pendingShares;
    mapping(address => uint256) public maxRedeem;
    uint256 public requestCount;
    uint256 public claimCount;

    constructor(address newAsset, address newShare) { asset = newAsset; share = newShare; }

    function requestRedeem(uint256 shares, address controller, address owner) external returns (uint256) {
        require(msg.sender == owner && owner == controller && shares > 0);
        require(FixtureShare(share).transferFrom(owner, address(this), shares));
        pendingShares[controller] += shares;
        return ++requestCount;
    }

    function finalize(address controller) external {
        maxRedeem[controller] += pendingShares[controller];
        pendingShares[controller] = 0;
    }

    function redeem(uint256 shares, address receiver, address controller) external returns (uint256 assets) {
        require(msg.sender == controller && receiver == controller && shares > 0);
        maxRedeem[controller] -= shares;
        assets = shares * 2;
        require(IFixtureOutput(asset).transfer(receiver, assets));
        ++claimCount;
    }
}
