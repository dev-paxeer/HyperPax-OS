// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {IPaymentStreams, STREAMS} from "./IPaxPrecompiles.sol";

/// @notice End-to-end test for PaymentStreams precompile (0x906).
///         Opens a native-token stream from sender -> payee at 1 wei/sec for
///         60s with a 100-wei cap. Writes streamId to .last_streams_run.env
///         so the shell wrapper can later assert accrued() grows and settle()
///         transfers funds.
///
/// NOTE: The streams precompile escrows the `cap` from the sender at open time
///       when token == address(0) (native HPX). The escrow is implemented in
///       x/streams via the bank module.
contract RunStreams is Script {
    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address sender = vm.addr(pk);
        address payee = vm.envOr("PAYEE", address(0xBEEF));
        console2.log("payer (sender):", sender);
        console2.log("payee:", payee);
        console2.log("block.timestamp at start:", block.timestamp);

        uint256 ratePerSecond = 1;          // 1 wei/sec
        uint64 startTime = uint64(block.timestamp);
        uint64 stopTime = startTime + 60;   // 60s duration
        uint256 cap = 100;                  // total cap 100 wei

        vm.startBroadcast(pk);
        uint256 streamId = IPaymentStreams(STREAMS).open(
            payee,
            address(0),       // native HPX
            ratePerSecond,
            startTime,
            stopTime,
            cap
        );
        vm.stopBroadcast();

        console2.log("opened streamId:", streamId);

        IPaymentStreams.Stream memory s = IPaymentStreams(STREAMS).getStream(streamId);
        require(s.id == streamId, "getStream: id mismatch");
        require(s.payer == sender, "getStream: payer mismatch");
        require(s.payee == payee, "getStream: payee mismatch");
        require(s.active, "getStream: not active");

        console2.log("getStream OK active=true ratePerSecond=", s.ratePerSecond);

        string memory line = string.concat(
            "STREAM_ID=", vm.toString(streamId), "\n",
            "PAYEE=", vm.toString(payee), "\n",
            "RATE_PER_SECOND=", vm.toString(ratePerSecond), "\n",
            "START_TIME=", vm.toString(uint256(startTime)), "\n",
            "STOP_TIME=", vm.toString(uint256(stopTime)), "\n",
            "CAP=", vm.toString(cap), "\n"
        );
        vm.writeFile("scripts/test_v20v21/.last_streams_run.env", line);
        console2.log("wrote .last_streams_run.env");
    }
}
