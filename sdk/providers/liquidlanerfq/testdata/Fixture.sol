// SPDX-License-Identifier: MIT
pragma solidity 0.8.37;

// Local integration fixture only. Administrative methods intentionally have no
// access controls; no fixture contract is suitable for deployment outside tests.
contract FixtureRFQAccount {
    struct Hash {
        uint256 offset;
        uint256 length;
        uint256 destination;
    }
    mapping(bytes32 => bool) public signatures;
    event Sign(bytes32 indexed digest, bytes32 indexed domainSeparator, bytes buffer, Hash[] plan);

    function isValidSignature(bytes32 digest, bytes calldata) external view returns (bytes4) {
        return signatures[digest] ? bytes4(0x1626ba7e) : bytes4(0xffffffff);
    }

    function sign(bytes32 domainSeparator, bytes calldata buffer, Hash[] calldata plan) external {
        require(msg.sender == address(this));
        uint256 size = buffer.length;
        require(size >= 32 && plan.length != 0);
        bytes memory collapsed = buffer;
        bytes32 structHash;
        for (uint256 i; i < plan.length; ++i) {
            uint256 offset = plan[i].offset;
            uint256 length = plan[i].length;
            uint256 destination = plan[i].destination;
            require(offset <= size && length <= size - offset && destination <= size - 32);
            assembly ("memory-safe") {
                let base := add(collapsed, 32)
                structHash := keccak256(add(base, offset), length)
                mstore(add(base, destination), structHash)
            }
        }
        bytes32 digest = keccak256(abi.encodePacked(hex"1901", domainSeparator, structHash));
        signatures[digest] = true;
        emit Sign(digest, domainSeparator, buffer, plan);
    }

    struct Action {
        address target;
        uint96 value;
        bytes data;
        bytes dataMask;
        bytes32[] proof;
    }
    event Manage(Action[] actions);
    bytes32 public root;
    function adapter() external view returns (address) { return address(this); }
    mapping(bytes32 => mapping(address => bool)) public hasRole;

    function configure(bytes32 newRoot, bytes32[] calldata leaves, address executor) external {
        root = newRoot;
        for (uint256 i; i < leaves.length; ++i) {
            hasRole[leaves[i]][executor] = true;
        }
    }

    function manage(Action[] calldata actions) external returns (bytes[] memory results) {
        results = new bytes[](actions.length);
        for (uint256 i; i < actions.length; ++i) {
            Action calldata action = actions[i];
            require(action.value == 0 && action.data.length == action.dataMask.length);
            bytes memory masked = new bytes(action.data.length);
            for (uint256 j; j < masked.length; ++j) {
                masked[j] = action.data[j] & ~action.dataMask[j];
            }
            bytes32 leaf = keccak256(
                bytes.concat(
                    keccak256(abi.encode(action.target, bytes1(0), keccak256(action.dataMask), keccak256(masked)))
                )
            );
            require(hasRole[leaf][msg.sender]);
            bytes32 computed = leaf;
            for (uint256 j; j < action.proof.length; ++j) {
                bytes32 other = action.proof[j];
                computed = computed < other
                    ? keccak256(abi.encodePacked(computed, other))
                    : keccak256(abi.encodePacked(other, computed));
            }
            require(computed == root);
            (bool success, bytes memory result) = action.target.call(action.data);
            require(success);
            results[i] = result;
        }
        emit Manage(actions);
    }
}

contract FixtureRFQToken {
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
    }

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

interface ISignature {
    function isValidSignature(bytes32, bytes calldata) external view returns (bytes4);
}

contract FixtureRFQReactor {
    struct Output {
        address token;
        uint256 amount;
        address recipient;
    }

    struct Request {
        address tokenIn;
        uint256 amountIn;
        Output[] outputs;
        uint256 deadline;
        uint256 nonce;
        address protocol;
    }

    struct Order {
        Request request;
        bytes swapperSignature;
        address swapper;
        address filler;
        Output[] outputs;
    }
    bytes32 constant OUTPUT_TYPEHASH = keccak256("Output(address token,uint256 amount,address recipient)");
    bytes32 constant REQUEST_TYPEHASH = keccak256(
        "Request(address tokenIn,uint256 amountIn,Output[] outputs,uint256 deadline,uint256 nonce,address protocol)Output(address token,uint256 amount,address recipient)"
    );
    mapping(address => mapping(uint256 => bool)) public isUsedNonce;
    event Fill(Order order);
    event InvalidateNonce(address indexed swapper, uint256 nonce);

    function invalidateNonce(uint256 nonce) external {
        isUsedNonce[msg.sender][nonce] = true;
        emit InvalidateNonce(msg.sender, nonce);
    }

    function eip712Domain()
        external
        view
        returns (bytes1, string memory, string memory, uint256, address, bytes32, uint256[] memory)
    {
        return (hex"0f", "Reactor", "1", block.chainid, address(this), bytes32(0), new uint256[](0));
    }

    function domain() public view returns (bytes32) {
        return keccak256(
            abi.encode(
                keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
                keccak256("Reactor"),
                keccak256("1"),
                block.chainid,
                address(this)
            )
        );
    }

    function fill(Order calldata order) external {
        Request calldata r = order.request;
        // Local fixture substitutes the protocol/filler caller for the upstream protocol signature.
        require(
            order.filler == msg.sender && r.protocol == msg.sender && block.timestamp <= r.deadline
                && !isUsedNonce[order.swapper][r.nonce]
        );
        bytes32[] memory hashes = new bytes32[](r.outputs.length);
        for (uint256 i; i < r.outputs.length; ++i) {
            hashes[i] = keccak256(abi.encode(OUTPUT_TYPEHASH, r.outputs[i]));
        }
        bytes32 requestHash = keccak256(
            abi.encode(
                REQUEST_TYPEHASH,
                r.tokenIn,
                r.amountIn,
                keccak256(abi.encodePacked(hashes)),
                r.deadline,
                r.nonce,
                r.protocol
            )
        );
        require(
            ISignature(order.swapper)
                .isValidSignature(keccak256(abi.encodePacked(hex"1901", domain(), requestHash)), order.swapperSignature)
            == 0x1626ba7e
        );
        require(r.outputs.length == order.outputs.length);
        isUsedNonce[order.swapper][r.nonce] = true;
        require(FixtureRFQToken(r.tokenIn).transferFrom(order.swapper, msg.sender, r.amountIn));
        for (uint256 i; i < order.outputs.length; ++i) {
            Output calldata o = order.outputs[i];
            require(
                o.token == r.outputs[i].token && o.recipient == r.outputs[i].recipient
                    && o.amount >= r.outputs[i].amount
            );
            require(FixtureRFQToken(o.token).transferFrom(msg.sender, o.recipient, o.amount));
        }
        emit Fill(order);
    }
}
