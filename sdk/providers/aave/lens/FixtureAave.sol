// SPDX-License-Identifier: MIT
pragma solidity 0.8.38;

interface IFixtureAaveToken {
    function mint(address owner, uint256 amount) external;
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

contract FixtureAaveToken {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    uint256 public totalSupply;
    function mint(address owner, uint256 amount) external { balanceOf[owner] += amount; totalSupply += amount; }
    function approve(address spender, uint256 amount) external returns (bool) {
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

// Local Anvil fixture only. Values model the Aave V3 getters used by the probe.
contract FixtureAave {
    uint256 public configuration = 7500 | (8000 << 16) | (6 << 48) | (1 << 56) | (1 << 58);
    uint256 public mode;
    uint128 public bitmap;
    uint256 public user;
    uint256 public collateral;
    uint256 public debt = 100e8;
    uint256 public available;
    uint256 public threshold = 8000;
    uint256 public cash = 1000e6;
    uint128 public virtualCash = 900e6;
    uint256 public price = 1e8;
    uint256 public totalSupply;
    bool public failPrice;
    bool public failBitmap;
    address public asset;
    address public otherReserve;
    uint256 public otherConfiguration;
    uint256 public rawDebt = 100e6;
    address public debtToken;
    address public aToken;

    function setDebtToken(address value) external { debtToken = value; }
    function setAToken(address value) external { aToken = value; }

    function setFailBitmap(bool value) external {
        failBitmap = value;
    }

    function setAsset(address value) external {
        asset = value;
    }

    function setState(uint256 c, uint256 d, uint256 a, uint256 t) external {
        collateral = c;
        debt = d;
        available = a;
        threshold = t;
    }

    function setConfiguration(uint256 value) external {
        configuration = value;
    }

    function setMode(uint256 value, uint128 bits) external {
        mode = value;
        bitmap = bits;
    }

    function setUser(uint256 value, address reserve, uint256 config) external {
        user = value;
        otherReserve = reserve;
        otherConfiguration = config;
    }

    function setLiquidity(uint256 value, uint128 virtualValue, uint256 supply) external {
        cash = value;
        virtualCash = virtualValue;
        totalSupply = supply;
    }

    function setPrice(uint256 value, bool fail) external {
        price = value;
        failPrice = fail;
    }

    function getConfiguration(address token) external view returns (uint256) {
        return token == asset ? configuration : otherConfiguration;
    }

    function getUserConfiguration(address) external view returns (uint256) {
        return user;
    }

    function getUserEMode(address) external view returns (uint256) {
        return mode;
    }

    function getEModeCategoryBorrowableBitmap(uint8) external view returns (uint128) {
        require(!failBitmap);
        return bitmap;
    }

    function getReserveAddressById(uint16 id) external view returns (address) {
        return id == 127 ? asset : otherReserve;
    }

    function getReserveAToken(address) external view returns (address) {
        return aToken == address(0) ? address(this) : aToken;
    }

    function getReserveVariableDebtToken(address) external view returns (address) {
        return debtToken == address(0) ? address(this) : debtToken;
    }

    function getVirtualUnderlyingBalance(address) external view returns (uint128) {
        return virtualCash;
    }

    function ADDRESSES_PROVIDER() external view returns (address) {
        return address(this);
    }

    function getPriceOracle() external view returns (address) {
        return address(this);
    }

    function getAssetPrice(address) external view returns (uint256) {
        require(!failPrice);
        return price;
    }

    function getUserAccountData(address) external view returns (uint256, uint256, uint256, uint256, uint256, uint256) {
        return (collateral, debt, available, threshold, 7500, _health());
    }

    function _health() private view returns (uint256) {
        return debt == 0 ? type(uint256).max : collateral * threshold / 10000 * 1e18 / debt;
    }

    function balanceOf(address owner) external view returns (uint256) {
        return owner == address(this) ? cash : rawDebt;
    }

    function supply(address token, uint256 amount, address owner, uint16 referral) external {
        require(token == asset && owner == msg.sender && referral == 0);
        if (token != address(this)) IFixtureAaveToken(token).transferFrom(owner, address(this), amount);
        if (aToken != address(0)) IFixtureAaveToken(aToken).mint(owner, amount);
        collateral += amount * 100;
        available += amount * 75;
    }

    function borrow(address token, uint256 amount, uint256 rate, uint16 referral, address owner) external {
        require(token == asset && owner == msg.sender && rate == 2 && referral == 0);
        require(amount <= cash && amount <= virtualCash);
        debt += amount * price / 1e6;
        rawDebt += amount;
        if (token != address(this)) IFixtureAaveToken(token).mint(owner, amount);
        if (debtToken != address(0)) IFixtureAaveToken(debtToken).mint(owner, amount);
    }
}

// Local Midas purchase double: normalized 18-decimal payment in, raw 18-decimal RWA out.
contract FixtureAavePurchase {
    address public immutable asset;
    address public immutable rwa;
    constructor(address newAsset, address newRwa) { asset = newAsset; rwa = newRwa; }
    function depositInstant(address token, uint256 amount, uint256 minimum, bytes32) external {
        require(token == asset && amount >= minimum);
        IFixtureAaveToken(asset).transferFrom(msg.sender, address(this), amount / 1e12);
        IFixtureAaveToken(rwa).mint(msg.sender, amount);
    }
}
