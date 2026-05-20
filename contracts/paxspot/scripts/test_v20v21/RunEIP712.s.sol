// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {IEIP712Helper, EIP712_HELPER} from "./IPaxPrecompiles.sol";

/// @notice Stateless verification of the EIP712Helper precompile (0x908).
///         Compares the precompile's output against Solidity's native
///         keccak256-based EIP-712 computation. They MUST match for the
///         precompile to be considered correct.
contract RunEIP712 is Script {
    bytes32 constant DOMAIN_TYPEHASH = keccak256(
        "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
    );

    function run() external view {
        string memory name = "Paxeer";
        string memory version = "1";
        uint256 chainId = 9000;
        address verifyingContract = address(0xdEad000000000000000000000000000000000000);

        // 1. domainSeparator must match Solidity's manual computation
        bytes32 expectedDomain = keccak256(
            abi.encode(
                DOMAIN_TYPEHASH,
                keccak256(bytes(name)),
                keccak256(bytes(version)),
                chainId,
                verifyingContract
            )
        );

        bytes32 actualDomain = IEIP712Helper(EIP712_HELPER).domainSeparator(
            name, version, chainId, verifyingContract
        );

        console2.log("expected domain separator:");
        console2.logBytes32(expectedDomain);
        console2.log("actual   domain separator:");
        console2.logBytes32(actualDomain);
        require(actualDomain == expectedDomain, "domainSeparator: mismatch");
        console2.log("domainSeparator: OK");

        // 2. hashTypedData must produce keccak256("\x19\x01" || domain || struct)
        bytes32 structHash = keccak256(bytes("hello-paxeer"));
        bytes32 expectedDigest = keccak256(
            abi.encodePacked(hex"1901", actualDomain, structHash)
        );
        bytes32 actualDigest = IEIP712Helper(EIP712_HELPER).hashTypedData(actualDomain, structHash);

        console2.log("expected digest:");
        console2.logBytes32(expectedDigest);
        console2.log("actual   digest:");
        console2.logBytes32(actualDigest);
        require(actualDigest == expectedDigest, "hashTypedData: mismatch");
        console2.log("hashTypedData: OK");

        console2.log("EIP712Helper precompile: ALL CHECKS PASS");
    }
}
