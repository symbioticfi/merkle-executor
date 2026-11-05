// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;

interface ClaimToken { function mint(address,uint256) external; }
// Local claim fixtures; administrative setters intentionally have no access control.
contract FixtureThreeFFactory {
    mapping(address => bool) public isRequest;
    function setRequest(address request) external { isRequest[request] = true; }
}
contract FixtureThreeFRequest {
    address public owner;
    address public token;
    uint128 public principal;
    bool public canWithdraw;
    function configure(address account, address asset, uint128 amount, bool ready) external {
        owner = account; token = asset; principal = amount; canWithdraw = ready;
    }
    function balancesOf(address account) external view returns (uint128, uint128) {
        return (account == owner ? principal : 0, 0);
    }
    function burnAll(address from, address receiver) external {
        require(canWithdraw && msg.sender == from && from == owner && receiver == owner);
        uint256 amount = principal;
        require(amount != 0);
        principal = 0;
        ClaimToken(token).mint(receiver, amount);
    }
}
