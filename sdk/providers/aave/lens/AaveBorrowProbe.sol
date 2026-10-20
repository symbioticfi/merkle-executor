// SPDX-License-Identifier: BUSL-1.1
// Copyright (c) 2026 Symbiotic
pragma solidity 0.8.37;

import {RecipePrefixLens} from "./RecipePrefixLens.sol";

interface IERC20 {
    function balanceOf(address) external view returns (uint256);
    function totalSupply() external view returns (uint256);
}

interface ILiquidityAavePool {
    function getConfiguration(address asset) external view returns (uint256);
    function getUserConfiguration(address account) external view returns (uint256);
    function getUserEMode(address account) external view returns (uint256);
    function getEModeCategoryBorrowableBitmap(uint8 id) external view returns (uint128);
    function getReserveAddressById(uint16 id) external view returns (address);
    function getReserveAToken(address asset) external view returns (address);
    function getReserveVariableDebtToken(address asset) external view returns (address);
    function getVirtualUnderlyingBalance(address asset) external view returns (uint128);
    function ADDRESSES_PROVIDER() external view returns (address);
    function getUserAccountData(address account)
        external
        view
        returns (uint256, uint256, uint256, uint256, uint256, uint256);
}

interface ILiquidityAaveProvider {
    function getPriceOracle() external view returns (address);
}

interface ILiquidityAaveOracle {
    function getAssetPrice(address asset) external view returns (uint256);
}

/// @notice eth_call code override only; never deployed or used as permission authority.
contract AaveBorrowProbe is RecipePrefixLens {
    function maxBorrow(address pool, address asset, address account, uint256 minHealthFactor)
        public
        view
        returns (uint256)
    {
        if (minHealthFactor < 1e18) revert();
        uint256 mode = ILiquidityAavePool(pool).getUserEMode(account);
        if (mode > type(uint8).max) return 0;
        if (mode > 0) {
            uint128 borrowable = ILiquidityAavePool(pool).getEModeCategoryBorrowableBitmap(uint8(mode));
            bool allowed;
            // Aave's uint128 category bitmap bounds the complete borrowable-reserve lookup.
            for (uint16 id; borrowable > 0; ++id) {
                if (borrowable & 1 > 0 && ILiquidityAavePool(pool).getReserveAddressById(id) == asset) {
                    allowed = true;
                    break;
                }
                borrowable >>= 1;
            }
            if (!allowed) return 0;
        }
        uint256 configuration = ILiquidityAavePool(pool).getConfiguration(asset);
        if (!_aaveActive(configuration) || configuration & (1 << 57) != 0 || configuration & (1 << 58) == 0) {
            return 0;
        }
        if (_restricted(pool, asset, ILiquidityAavePool(pool).getUserConfiguration(account), configuration)) return 0;
        (uint256 collateral, uint256 debt, uint256 available, uint256 threshold,,) =
            ILiquidityAavePool(pool).getUserAccountData(account);
        uint256 headroom = _sub(_mulDiv(collateral * threshold / 10_000, 1e18, minHealthFactor), debt);
        uint256 base = _min(headroom, available);
        if (base == 0) return 0;
        uint256 price = ILiquidityAaveOracle(
                ILiquidityAaveProvider(ILiquidityAavePool(pool).ADDRESSES_PROVIDER()).getPriceOracle()
            )
            .getAssetPrice(asset);
        if (price == 0) return 0;
        uint256 decimals = configuration >> 48 & 0xff;
        if (decimals > 36) revert();
        uint256 unit = 10 ** decimals;
        // The recipe admits at most one raw debt unit of indexed-token rounding.
        uint256 amount = _sub(_mulDiv(base, unit, price), 1);
        address aToken = ILiquidityAavePool(pool).getReserveAToken(asset);
        amount = _min(
            amount, _min(IERC20(asset).balanceOf(aToken), ILiquidityAavePool(pool).getVirtualUnderlyingBalance(asset))
        );
        uint256 borrowCap = configuration >> 80 & ((1 << 36) - 1);
        if (borrowCap != 0) {
            uint256 totalDebt = IERC20(ILiquidityAavePool(pool).getReserveVariableDebtToken(asset)).totalSupply();
            amount = _min(amount, _sub(_sub(borrowCap * unit, totalDebt), 1));
        }
        return amount;
    }

    function _aaveActive(uint256 configuration) internal pure returns (bool) {
        return configuration & (1 << 56) > 0 && configuration & (1 << 60) == 0;
    }

    function _restricted(address pool, address asset, uint256 user, uint256 assetConfiguration)
        internal
        view
        returns (bool restricted)
    {
        for (uint16 i; user > 0 && i < 128; ++i) {
            uint256 flags = user & 3;
            user >>= 2;
            if (flags == 0) continue;
            address reserve = ILiquidityAavePool(pool).getReserveAddressById(i);
            uint256 configuration =
                reserve == asset ? assetConfiguration : ILiquidityAavePool(pool).getConfiguration(reserve);
            if (flags & 2 > 0) {
                if (configuration >> 212 & ((1 << 40) - 1) > 0) restricted = true;
            }
            if (flags & 1 > 0 && configuration & (1 << 62) > 0) restricted = true;
        }
        if (assetConfiguration & (1 << 62) > 0) restricted = true;
    }

    function _min(uint256 a, uint256 b) private pure returns (uint256) {
        return a < b ? a : b;
    }

    function _sub(uint256 a, uint256 b) private pure returns (uint256) {
        return a > b ? a - b : 0;
    }

    // Full-precision floor division, adapted from OpenZeppelin Math (MIT); see README.md.
    function _mulDiv(uint256 x, uint256 y, uint256 denominator) internal pure returns (uint256 result) {
        unchecked {
            uint256 high;
            uint256 low;
            assembly ("memory-safe") {
                let mm := mulmod(x, y, not(0))
                low := mul(x, y)
                high := sub(sub(mm, low), lt(mm, low))
            }

            if (high == 0) {
                return low / denominator;
            }

            if (denominator <= high) {
                revert();
            }

            uint256 remainder;
            assembly ("memory-safe") {
                remainder := mulmod(x, y, denominator)

                high := sub(high, gt(remainder, low))
                low := sub(low, remainder)
            }

            uint256 twos = denominator & (0 - denominator);
            assembly ("memory-safe") {
                denominator := div(denominator, twos)

                low := div(low, twos)

                twos := add(div(sub(0, twos), twos), 1)
            }

            low |= high * twos;

            uint256 inverse = (3 * denominator) ^ 2;

            inverse *= 2 - denominator * inverse; // inverse mod 2⁸
            inverse *= 2 - denominator * inverse; // inverse mod 2¹⁶
            inverse *= 2 - denominator * inverse; // inverse mod 2³²
            inverse *= 2 - denominator * inverse; // inverse mod 2⁶⁴
            inverse *= 2 - denominator * inverse; // inverse mod 2¹²⁸
            inverse *= 2 - denominator * inverse; // inverse mod 2²⁵⁶

            result = low * inverse;
            return result;
        }
    }
}
