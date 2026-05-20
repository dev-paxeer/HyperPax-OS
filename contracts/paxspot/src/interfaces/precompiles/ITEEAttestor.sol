// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title ITEEAttestor
/// @notice Interface for the TEE Attestation precompile at 0x0907.
/// @dev    Verifies hardware attestation quotes (Intel TDX, AMD SEV-SNP,
///         NVIDIA H100 confidential, Intel SGX DCAP) on-chain in milliseconds
///         instead of millions of gas. Backed by a small registry of trusted
///         root certificates governed via `x/attestor`.
///
///         Trusted roots are loaded post-upgrade via gov-proposal. Until roots
///         exist for a family, `verify(...)` reverts with `ErrNoRootsLoaded`.
interface ITEEAttestor {
    /// @dev MUST stay in sync with `x/attestor/types/keys.go::Family*` constants
    ///      and with `precompiles/teeattestor/abi.json`.
    enum Family {
        INTEL_TDX,    // 0
        AMD_SEV_SNP,  // 1
        NVIDIA_H100,  // 2
        INTEL_SGX     // 3
    }

    struct Attestation {
        Family  family;
        bytes32 mrtd;        // measurement (TDX) / launch digest (SNP) / image hash (NVIDIA)
        bytes32 reportData;  // 32 bytes app-provided data committed inside the quote
        uint64  timestamp;   // unix seconds, per the TEE platform's clock
        bool    debug;       // TEE in debug mode (untrusted unless Params.DebugAllowed)
    }

    /// @notice Verify a raw attestation quote.
    /// @dev    Reverts on bad signature, broken cert chain, untrusted root,
    ///         or stale timestamp (older than `Params.MaxAttestationAge`).
    function verify(Family family, bytes calldata quote)
        external view returns (Attestation memory);

    /// @notice Convenience: verify and assert `reportData` matches `expectedReportData`.
    ///         Saves a re-hash + comparison in the caller.
    function verifyAndExpect(Family family, bytes calldata quote, bytes32 expectedReportData)
        external view returns (Attestation memory);

    /// @notice Trusted root pubkey/cert for a TEE family at the given index.
    function rootOf(Family family, uint256 index) external view returns (bytes memory);

    /// @notice Number of trusted roots loaded for the given family.
    function rootCount(Family family) external view returns (uint256);

    // ── Events ──────────────────────────────────────────────────────────────

    /// @notice Emitted on every successful verification. Used by attested-compute
    ///         indexers to track which workloads have been validated on-chain.
    event AttestationVerified(
        Family  indexed family,
        bytes32 indexed mrtd,
        bytes32 reportData,
        uint64  timestamp,
        bool    debug
    );
}
