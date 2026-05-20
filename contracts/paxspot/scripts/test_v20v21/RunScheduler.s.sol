// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {Script, console2} from "forge-std/Script.sol";
import {IScheduler, SCHEDULER} from "./IPaxPrecompiles.sol";
import {JobTarget} from "./JobTarget.sol";

/// @notice End-to-end state-mutating test for the Scheduler precompile (0x905).
///         1. Deploys a JobTarget
///         2. Schedules a `poke()` call ~5 blocks in the future, paying a deposit
///         3. Verifies the job is queued via getJob()
///         4. Polling logic outside (the shell wrapper) handles the wait
contract RunScheduler is Script {
    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address sender = vm.addr(pk);
        console2.log("sender:", sender);
        console2.log("block.number at start:", block.number);

        vm.startBroadcast(pk);

        JobTarget target = new JobTarget();
        console2.log("JobTarget deployed at:", address(target));

        // Schedule the poke() ~5 blocks in the future, gas limit 200k.
        bytes memory cd = abi.encodeWithSelector(JobTarget.poke.selector);
        uint64 fireAt = uint64(block.number + 5);
        uint64 gasLimit = 200_000;
        uint256 deposit = 1 ether; // generous; precompile refunds the unused portion

        uint256 jobId = IScheduler(SCHEDULER).schedule{value: deposit}(
            address(target), cd, fireAt, gasLimit
        );

        vm.stopBroadcast();

        console2.log("scheduled jobId:", jobId);
        console2.log("scheduled fireAt block:", fireAt);

        // Sanity: read it back
        IScheduler.Job memory j = IScheduler(SCHEDULER).getJob(jobId);
        require(j.id == jobId, "getJob: id mismatch");
        require(j.target == address(target), "getJob: target mismatch");
        require(j.executeAtBlock == fireAt, "getJob: fireAt mismatch");
        require(j.active, "getJob: not active");

        console2.log("getJob returned active=true, target/fireAt match");

        // Write the addresses + jobId to a file so the shell wrapper can poll.
        // forge-std default: scripts/ is read-write per foundry.toml.
        string memory line = string.concat(
            "JOB_ID=", vm.toString(jobId), "\n",
            "JOB_TARGET=", vm.toString(address(target)), "\n",
            "FIRE_AT_BLOCK=", vm.toString(uint256(fireAt)), "\n"
        );
        vm.writeFile("scripts/test_v20v21/.last_scheduler_run.env", line);
        console2.log("wrote .last_scheduler_run.env");
    }
}
