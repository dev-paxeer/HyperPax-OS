// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title Paxeer v20/v21 Precompile Interfaces
/// @notice Slim Solidity interfaces for the Paxeer-native precompiles. ABIs
///         are mirrored from `precompiles/{scheduler,streams,teeattestor,eip712}/abi.json`.

interface IScheduler {
    struct Job {
        uint256 id;
        address creator;
        address target;
        bytes callData;
        uint64 executeAtBlock;
        uint64 gasLimit;
        uint256 deposit;
        bool active;
    }

    function schedule(
        address target,
        bytes calldata callData,
        uint64 executeAtBlock,
        uint64 gasLimit
    )
        external
        payable
        returns (uint256 jobId);

    function cancel(uint256 jobId) external;
    function reschedule(uint256 jobId, uint64 newExecuteAtBlock) external;
    function getJob(uint256 jobId) external view returns (Job memory job);
    function pending(address creator) external view returns (uint256[] memory ids);
}

interface IPaymentStreams {
    struct Stream {
        uint256 id;
        address payer;
        address payee;
        address token;
        uint256 ratePerSecond;
        uint256 cap;
        uint64 startTime;
        uint64 stopTime;
        uint256 settled;
        bool active;
    }

    function open(
        address payee,
        address token,
        uint256 ratePerSecond,
        uint64 startTime,
        uint64 stopTime,
        uint256 cap
    )
        external
        returns (uint256 streamId);

    function settle(uint256 streamId) external;
    function close(uint256 streamId) external;
    function updateRate(uint256 streamId, uint256 newRate) external;
    function accrued(uint256 streamId) external view returns (uint256 amount);
    function getStream(uint256 streamId) external view returns (Stream memory s);
}

interface ITEEAttestor {
    function verify(uint8 teeType, bytes calldata envelope) external view returns (bool ok);
    function verifyAndExpect(uint8 teeType, bytes calldata envelope, bytes32 expectedMrEnclave)
        external
        view
        returns (bool ok);
    function rootOf(uint8 teeType, uint256 index) external view returns (bytes memory rootDer);
    function rootCount(uint8 teeType) external view returns (uint256 count);
}

interface IEIP712Helper {
    function hashTypedData(bytes32 domainSeparator, bytes32 structHash) external pure returns (bytes32 digest);
    function domainSeparator(string calldata name, string calldata version, uint256 chainId, address verifyingContract)
        external
        pure
        returns (bytes32 separator);
    function recoverTypedSigner(bytes32 domainSeparator, bytes32 structHash, bytes calldata signature)
        external
        pure
        returns (address signer);
}

address constant SCHEDULER = address(0x0000000000000000000000000000000000000905);
address constant STREAMS = address(0x0000000000000000000000000000000000000906);
address constant TEE_ATTESTOR = address(0x0000000000000000000000000000000000000907);
address constant EIP712_HELPER = address(0x0000000000000000000000000000000000000908);
