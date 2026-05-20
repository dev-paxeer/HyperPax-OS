// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {ITEEAttestor, TEE_ATTESTOR} from "./IPaxPrecompiles.sol";

/// @notice Read-only sanity test for TEEAttestor (0x907). Without TEE roots
///         loaded (which is the genesis state), `rootCount` is 0 and `verify`
///         returns false. Both behaviours prove the precompile is wired and
///         querying x/attestor's store correctly. Full attestation tests
///         require gov-uploaded root certs and live in Go unit tests.
contract RunAttestor is Script {
    function run() external view {
        // Iterate the 4 supported TEE types (SGX=0, SEV-SNP=1, TDX=2, NITRO=3)
        for (uint8 t = 0; t < 4; t++) {
            uint256 n = ITEEAttestor(TEE_ATTESTOR).rootCount(t);
            console2.log("teeType=", t, " rootCount=", n);
        }

        // verify with empty envelope should NOT revert (it returns false).
        bool ok = ITEEAttestor(TEE_ATTESTOR).verify(0, hex"");
        require(!ok, "verify(empty) should be false");
        console2.log("verify(empty) returned false as expected");

        console2.log("TEEAttestor precompile: reachable, returns deterministic results");
    }
}
