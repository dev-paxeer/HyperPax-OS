// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IEIP712Helper
/// @notice Interface for the EIP-712 helper precompile at 0x0908.
/// @dev    Native EIP-712 typed-data hashing + signer recovery. Saves ~5k gas
///         on every signed-message verification across the agent stack
///         (payment-channel sigs, off-chain orders, attestation envelopes).
///         Stateless; safe under STATICCALL.
interface IEIP712Helper {
    /// @notice Compute the EIP-712 digest given a domain separator and a struct hash.
    /// @return digest = keccak256("\\x19\\x01" || domainSeparator || structHash)
    function hashTypedData(bytes32 domainSeparator, bytes32 structHash)
        external pure returns (bytes32 digest);

    /// @notice Build the EIP-712 domain separator from primitive fields.
    /// @dev    separator = keccak256(abi.encode(
    ///                 keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
    ///                 keccak256(bytes(name)),
    ///                 keccak256(bytes(version)),
    ///                 chainId,
    ///                 verifyingContract))
    function domainSeparator(
        string calldata name,
        string calldata version,
        uint256 chainId,
        address verifyingContract
    ) external pure returns (bytes32 separator);

    /// @notice Recover the signer of an EIP-712-signed message in a single call
    ///         (avoids two precompile hops: hashTypedData + ecrecover).
    /// @dev    `signature` is the standard 65-byte (r || s || v) Ethereum
    ///         signature. Returns `address(0)` on bad signature.
    function recoverTypedSigner(
        bytes32 domainSeparator_,
        bytes32 structHash,
        bytes calldata signature
    ) external view returns (address signer);
}
