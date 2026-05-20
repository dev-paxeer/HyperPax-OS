// Copyright PaxLabs Ltd.(Paxeer Network)
// Paxeer Network Non-Commercial License 1.0 (ENCL-1.0)(https://github.com/Paxeer-Network/hyperpaxeer-os/blob/main/LICENSE_FAQ.md)

package keeper

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"golang.org/x/exp/maps"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	channelkeeper "github.com/cosmos/ibc-go/v7/modules/core/04-channel/keeper"

	bankprecompile "github.com/evmos/evmos/v18/precompiles/bank"
	"github.com/evmos/evmos/v18/precompiles/bech32"
	distprecompile "github.com/evmos/evmos/v18/precompiles/distribution"
	paxeip712 "github.com/evmos/evmos/v18/precompiles/eip712"
	ics20precompile "github.com/evmos/evmos/v18/precompiles/ics20"
	"github.com/evmos/evmos/v18/precompiles/p256"
	paxclearing "github.com/evmos/evmos/v18/precompiles/paxspot/clearing"
	paxoracle "github.com/evmos/evmos/v18/precompiles/paxspot/oracle"
	paxorob "github.com/evmos/evmos/v18/precompiles/paxspot/orob"
	paxpofq "github.com/evmos/evmos/v18/precompiles/paxspot/pofq"
	paxscheduler "github.com/evmos/evmos/v18/precompiles/scheduler"
	stakingprecompile "github.com/evmos/evmos/v18/precompiles/staking"
	paxstreams "github.com/evmos/evmos/v18/precompiles/streams"
	paxteeattestor "github.com/evmos/evmos/v18/precompiles/teeattestor"
	vestingprecompile "github.com/evmos/evmos/v18/precompiles/vesting"
	attestorkeeper "github.com/evmos/evmos/v18/x/attestor/keeper"
	erc20Keeper "github.com/evmos/evmos/v18/x/erc20/keeper"
	transferkeeper "github.com/evmos/evmos/v18/x/ibc/transfer/keeper"
	paxoraclekeeper "github.com/evmos/evmos/v18/x/paxoracle/keeper"
	schedulerkeeper "github.com/evmos/evmos/v18/x/scheduler/keeper"
	stakingkeeper "github.com/evmos/evmos/v18/x/staking/keeper"
	streamskeeper "github.com/evmos/evmos/v18/x/streams/keeper"
	vestingkeeper "github.com/evmos/evmos/v18/x/vesting/keeper"
)

// AvailablePrecompiles returns the list of all available precompiled contracts.
// NOTE: this should only be used during initialization of the Keeper.
func AvailablePrecompiles(
	stakingKeeper stakingkeeper.Keeper,
	distributionKeeper distributionkeeper.Keeper,
	bankKeeper bankkeeper.Keeper,
	erc20Keeper erc20Keeper.Keeper,
	vestingKeeper vestingkeeper.Keeper,
	authzKeeper authzkeeper.Keeper,
	transferKeeper transferkeeper.Keeper,
	channelKeeper channelkeeper.Keeper,
	paxOracleKeeper paxoraclekeeper.Keeper,
	schedulerKeeper schedulerkeeper.Keeper,
	streamsKeeper streamskeeper.Keeper,
	attestorKeeper attestorkeeper.Keeper,
) map[common.Address]vm.PrecompiledContract {
	// Clone the mapping from the latest EVM fork.
	precompiles := maps.Clone(vm.PrecompiledContractsBerlin)

	// secp256r1 precompile as per EIP-7212
	p256Precompile := &p256.Precompile{}

	bech32Precompile, err := bech32.NewPrecompile(6000)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bech32 precompile: %w", err))
	}

	stakingPrecompile, err := stakingprecompile.NewPrecompile(stakingKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate staking precompile: %w", err))
	}

	distributionPrecompile, err := distprecompile.NewPrecompile(distributionKeeper, stakingKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate distribution precompile: %w", err))
	}

	ibcTransferPrecompile, err := ics20precompile.NewPrecompile(stakingKeeper, transferKeeper, channelKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate ICS20 precompile: %w", err))
	}

	vestingPrecompile, err := vestingprecompile.NewPrecompile(vestingKeeper, authzKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate vesting precompile: %w", err))
	}

	bankPrecompile, err := bankprecompile.NewPrecompile(bankKeeper, erc20Keeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bank precompile: %w", err))
	}

	// Stateless precompiles
	precompiles[bech32Precompile.Address()] = bech32Precompile
	precompiles[p256Precompile.Address()] = p256Precompile

	// Stateful precompiles
	precompiles[stakingPrecompile.Address()] = stakingPrecompile
	precompiles[distributionPrecompile.Address()] = distributionPrecompile
	precompiles[vestingPrecompile.Address()] = vestingPrecompile
	precompiles[ibcTransferPrecompile.Address()] = ibcTransferPrecompile
	precompiles[bankPrecompile.Address()] = bankPrecompile

	// PaxSpot precompiles
	orobPrecompile, err := paxorob.NewPrecompile()
	if err != nil {
		panic(fmt.Errorf("failed to instantiate OROB precompile: %w", err))
	}

	clearingPrecompile, err := paxclearing.NewPrecompile()
	if err != nil {
		panic(fmt.Errorf("failed to instantiate BatchClearing precompile: %w", err))
	}

	oraclePrecompile, err := paxoracle.NewPrecompile(paxOracleKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate OracleAggregator precompile: %w", err))
	}

	pofqPrecompile, err := paxpofq.NewPrecompile()
	if err != nil {
		panic(fmt.Errorf("failed to instantiate PoFQ precompile: %w", err))
	}

	precompiles[orobPrecompile.Address()] = orobPrecompile
	precompiles[clearingPrecompile.Address()] = clearingPrecompile
	precompiles[oraclePrecompile.Address()] = oraclePrecompile
	precompiles[pofqPrecompile.Address()] = pofqPrecompile

	// Scheduler precompile (0x0905, v20-agent-foundations).
	schedulerPrecompile, err := paxscheduler.NewPrecompile(schedulerKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate Scheduler precompile: %w", err))
	}
	precompiles[schedulerPrecompile.Address()] = schedulerPrecompile

	// v21-agent-payments precompiles. Activation is gated by the v21agent
	// upgrade handler appending these addresses to EvmParams.ActivePrecompiles;
	// being in this map only means the keeper CAN dispatch to them, not that
	// pre-upgrade EVM calls will find them.

	// PaymentStreams precompile (0x0906).
	streamsPrecompile, err := paxstreams.NewPrecompile(streamsKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate PaymentStreams precompile: %w", err))
	}
	precompiles[streamsPrecompile.Address()] = streamsPrecompile

	// TEEAttestor precompile (0x0907).
	teeAttestorPrecompile, err := paxteeattestor.NewPrecompile(attestorKeeper)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate TEEAttestor precompile: %w", err))
	}
	precompiles[teeAttestorPrecompile.Address()] = teeAttestorPrecompile

	// EIP712Helper precompile (0x0908). Stateless — no keeper.
	eip712Precompile, err := paxeip712.NewPrecompile()
	if err != nil {
		panic(fmt.Errorf("failed to instantiate EIP712Helper precompile: %w", err))
	}
	precompiles[eip712Precompile.Address()] = eip712Precompile

	return precompiles
}

// WithPrecompiles sets the available precompiled contracts.
func (k *Keeper) WithPrecompiles(precompiles map[common.Address]vm.PrecompiledContract) *Keeper {
	if k.precompiles != nil {
		panic("available precompiles map already set")
	}

	if len(precompiles) == 0 {
		panic("empty precompiled contract map")
	}

	k.precompiles = precompiles
	return k
}

// Precompiles returns the subset of the available precompiled contracts that
// are active given the current parameters.
func (k Keeper) Precompiles(
	activePrecompiles ...common.Address,
) map[common.Address]vm.PrecompiledContract {
	activePrecompileMap := make(map[common.Address]vm.PrecompiledContract)

	for _, address := range activePrecompiles {
		precompile, ok := k.precompiles[address]
		if !ok {
			panic(fmt.Sprintf("precompiled contract not initialized: %s", address))
		}

		activePrecompileMap[address] = precompile
	}

	return activePrecompileMap
}

// AddEVMExtensions adds the given precompiles to the list of active precompiles in the EVM parameters
// and to the available precompiles map in the Keeper. This function returns an error if
// the precompiles are invalid or duplicated.
func (k *Keeper) AddEVMExtensions(ctx sdk.Context, precompiles ...vm.PrecompiledContract) error {
	params := k.GetParams(ctx)

	addresses := make([]string, len(precompiles))
	precompilesMap := maps.Clone(k.precompiles)

	for i, precompile := range precompiles {
		// add to active precompiles
		address := precompile.Address()
		addresses[i] = address.String()

		// add to available precompiles, but check for duplicates
		if _, ok := precompilesMap[address]; ok {
			return fmt.Errorf("precompile already registered: %s", address)
		}
		precompilesMap[address] = precompile
	}

	params.ActivePrecompiles = append(params.ActivePrecompiles, addresses...)

	// NOTE: the active precompiles are sorted and validated before setting them
	// in the params
	if err := k.SetParams(ctx, params); err != nil {
		return err
	}

	// update the pointer to the map with the newly added EVM Extensions
	k.precompiles = precompilesMap
	return nil
}

// IsAvailablePrecompile returns true if the given precompile address is contained in the
// EVM keeper's available precompiles map.
func (k Keeper) IsAvailablePrecompile(address common.Address) bool {
	_, ok := k.precompiles[address]
	return ok
}

// GetAvailablePrecompileAddrs returns the list of available precompile addresses.
//
// NOTE: uses index based approach instead of append because it's supposed to be faster.
// Check https://stackoverflow.com/questions/21362950/getting-a-slice-of-keys-from-a-map.
func (k Keeper) GetAvailablePrecompileAddrs() []common.Address {
	addresses := make([]common.Address, len(k.precompiles))
	i := 0

	//#nosec G705 -- two operations in for loop here are fine
	for address := range k.precompiles {
		addresses[i] = address
		i++
	}

	sort.Slice(addresses, func(i, j int) bool {
		return bytes.Compare(addresses[i].Bytes(), addresses[j].Bytes()) == -1
	})

	return addresses
}
