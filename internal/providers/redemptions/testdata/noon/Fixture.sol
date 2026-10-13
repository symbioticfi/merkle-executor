// SPDX-License-Identifier: MIT
pragma solidity 0.8.28;

import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";

// Local-only token and oracle fixtures. RedeemHandlerV2 itself is the unmodified
// source-verified production contract compiled alongside its pinned dependencies.
contract FixtureNoonToken is ERC20Burnable {
    uint8 private immutable tokenDecimals;
    constructor(uint8 decimals_) ERC20("Fixture", "FIX") { tokenDecimals = decimals_; }
    function decimals() public view override returns (uint8) { return tokenDecimals; }
    function mint(address recipient, uint256 amount) external { _mint(recipient, amount); }
}

contract FixtureNoonOracle {
    int256 public price = 1e8;
    uint256 public updatedAt = block.timestamp;
    function set(int256 value, uint256 updated) external { price = value; updatedAt = updated; }
    function latestRoundData() external view returns (uint80, int256, uint256, uint256, uint80) {
        return (1, price, updatedAt, updatedAt, 1);
    }
}
