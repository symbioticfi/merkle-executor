// SPDX-License-Identifier: BUSL-1.1
// Copyright (c) 2026 Symbiotic
pragma solidity 0.8.37;

interface IPrefixAccount {
    struct Action { address target; uint96 value; bytes data; bytes dataMask; bytes32[] proof; }
    function manage(Action[] calldata actions) external returns (bytes[] memory);
}

/// @notice Exact supported prefix of packed Recipes at the revision in sdk/abi/source.json.
/// @dev eth_call code override at the original executor preserves msg.sender and
/// tx.origin for external reads/effects. Original execute is still authoritative.
contract RecipePrefixLens {
    error InvalidReference(); error InvalidQuery(); error InvalidCall();
    error InvalidAction(); error InvalidRuntime(); error CheckFailed(); error ApprovalFailed();
    error AddressEmptyCode(address target);
    // Same transient reentrancy slot as the pinned OpenZeppelin executor.
    modifier guarded() {
        bytes32 slot=0x9b779b17422d0df92223018b32b4d1fa46e071723d6817e2486d003becc55f00;
        uint256 entered; assembly ("memory-safe") { entered:=tload(slot) }
        if(entered!=0) revert InvalidCall();
        assembly ("memory-safe") { tstore(slot,1) }
        _;
        assembly ("memory-safe") { tstore(slot,0) }
    }
    struct State { uint256[] values; bool[] needed; uint256[] inputs; bool[] known; bytes[] anchors; uint256 index; }

    function resolve(address account, bytes[] calldata queries, bytes[] calldata steps,
        uint256[] calldata inputs, bool[] calldata known, bytes[] calldata anchors,
        bytes[] calldata runtime, uint256[] calldata indices) external guarded returns (uint256[] memory)
    {
        State memory s = State(new uint256[](steps.length), new bool[](steps.length), inputs, known, anchors, 0);
        uint256 end=0; uint256 runtimeIndex=0;
        for (uint256 i; i < indices.length; ++i) {
            if (indices[i] >= steps.length) revert InvalidReference();
            s.needed[indices[i]] = true;
            if (indices[i] + 1 > end) end = indices[i] + 1;
        }
        for (; s.index < end; ++s.index) {
            uint8 op = uint8(steps[s.index][0]); bytes calldata data = steps[s.index][1:];
            if (op == 9) {
                bytes calldata payload = runtime[runtimeIndex++];
                (uint256 gate, bytes32[] memory leaves, uint256[][] memory bindings) = abi.decode(data,(uint256,bytes32[],uint256[][]));
                if (_value(gate,steps,s)==0) continue;
                IPrefixAccount.Action[] memory actions = abi.decode(payload,(IPrefixAccount.Action[]));
                if (actions.length != leaves.length || actions.length != bindings.length) revert InvalidRuntime();
                for (uint256 i; i<actions.length; ++i) { _bind(actions[i].data,bindings[i],steps,s); _check(actions[i],leaves[i]); }
                IPrefixAccount(account).manage(actions);
            } else if (op == 10) {
                (uint256 amount,address token,address spender,bytes32 leaf)=abi.decode(data,(uint256,address,address,bytes32));
                amount=_value(amount,steps,s);
                (bytes memory mask,bytes32[] memory proof)=abi.decode(runtime[runtimeIndex++],(bytes,bytes32[]));
                bytes memory allowanceData=abi.encodeWithSelector(bytes4(keccak256("allowance(address,address)")),account,spender);
                uint256 length=1;
                if(amount!=0 && abi.decode(_call(token,allowanceData,true),(uint256))!=0) length=2;
                IPrefixAccount.Action[] memory actions=new IPrefixAccount.Action[](length);
                for(uint256 i; i<length; ++i) {
                    actions[i]=IPrefixAccount.Action(token,0,abi.encodeWithSelector(bytes4(keccak256("approve(address,uint256)")),spender,length==1 || i>0 ? amount : 0),mask,proof);
                    _check(actions[i],leaf);
                }
                bytes[] memory responses=IPrefixAccount(account).manage(actions);
                for(uint256 i; i<responses.length; ++i) { if(responses[i].length>0 && !abi.decode(responses[i],(bool))) revert ApprovalFailed(); }
            } else if (op == 11) {
                (address target,bytes memory callData,uint256[] memory bindings)=abi.decode(data,(address,bytes,uint256[]));
                _bind(callData,bindings,steps,s); _call(target,callData,false);
            } else if (op == 0) {
                if (!s.needed[s.index]) continue;
                (address target,bytes memory callData,uint256[] memory bindings,uint256 offset)=abi.decode(queries[abi.decode(data,(uint256))],(address,bytes,uint256[],uint256));
                _bind(callData,bindings,steps,s); bytes memory result=_call(target,callData,true);
                if(offset+32>result.length) revert InvalidQuery();
                uint256 value; assembly("memory-safe") { value:=mload(add(add(result,32),offset)) }
                s.values[s.index]=value;
            } else {
                if (op>8) revert InvalidReference();
                if (op<=6 && !s.needed[s.index]) continue;
                (uint256 a,uint256 b)=abi.decode(data,(uint256,uint256)); a=_value(a,steps,s); b=_value(b,steps,s);
                if(op==1) s.values[s.index]=a+b;
                else if(op==2) s.values[s.index]=a-b;
                else if(op==3) s.values[s.index]=a>b ? a-b : 0;
                else if(op==4) s.values[s.index]=a<b ? a : b;
                else if(op==5) s.values[s.index]=a>b ? a : b;
                else if(op==6) s.values[s.index]=a*b/_value(abi.decode(data[64:],(uint256)),steps,s);
                else if(op==7 ? a!=b : a>b) revert CheckFailed();
            }
        }
        return s.values;
    }
    function _value(uint256 word,bytes[] calldata steps,State memory s) private pure returns(uint256) {
        uint8 source=uint8(word>>216); uint256 value=uint216(word);
        if(source==0) return value;
        if(source==1) {
            if(value>=s.inputs.length) revert InvalidReference();
            if(s.known[value]) return s.inputs[value];
            if(s.anchors[value].length==0) revert InvalidReference();
            return _value(abi.decode(s.anchors[value],(uint256)),steps,s);
        }
        if(source!=2 || value>=s.index || uint8(steps[value][0])>6 || !s.needed[value]) revert InvalidReference();
        return s.values[value];
    }
    function _bind(bytes memory data,uint256[] memory bindings,bytes[] calldata steps,State memory s) private pure {
        uint256 end=4;
        for(uint256 i;i<bindings.length;++i) {
            uint256 offset=bindings[i]>>224; if(offset<end) revert InvalidCall(); end=offset+32; if(end>data.length) revert InvalidCall();
            uint256 value=_value(bindings[i],steps,s); assembly("memory-safe") { mstore(add(add(data,32),offset),value) }
        }
    }
    function _call(address target,bytes memory data,bool read) private returns(bytes memory result) {
        bool success;
        if(read) (success,result)=target.staticcall(data); else (success,result)=target.call(data);
        if(!success) { assembly("memory-safe") { revert(add(result,32),mload(result)) } }
        if(result.length==0 && target.code.length==0) revert AddressEmptyCode(target);
    }
    function _check(IPrefixAccount.Action memory action,bytes32 leaf) private pure {
        if(action.value!=0 || action.data.length!=action.dataMask.length) revert InvalidAction();
        bytes memory masked=new bytes(action.data.length);
        for(uint256 i;i<masked.length;++i) masked[i]=action.data[i] & ~action.dataMask[i];
        if(keccak256(bytes.concat(keccak256(abi.encode(action.target,bytes1(0),keccak256(action.dataMask),keccak256(masked)))))!=leaf) revert InvalidAction();
    }
}
