package mint

import (
	"time"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/mint/keeper"
	"github.com/cosmos/cosmos-sdk/x/mint/types"
)

// BeginBlocker mints new tokens for the previous block.
func BeginBlocker(ctx sdk.Context, k keeper.Keeper) {
	defer telemetry.ModuleMeasureSince(types.ModuleName, time.Now(), telemetry.MetricKeyBeginBlocker)

	// fetch stored minter & params
	minter := k.GetMinter(ctx)
	params := k.GetParams(ctx)

	// maxtoken supply
	MaxTokenSupply := sdk.NewIntFromBigInt(params.MaxTokenSupply.BigInt())

	// recalculate inflation rate
	totalStakingSupply := k.StakingTokenSupply(ctx)

	if totalStakingSupply.LT(MaxTokenSupply) {
		bondedRatio := k.BondedRatio(ctx)
		minter.Inflation = minter.NextInflationRate(params, bondedRatio)
		minter.AnnualProvisions = minter.NextAnnualProvisions(params, totalStakingSupply)
		k.SetMinter(ctx, minter)

		// mint coins, update supply
		mintedCoin := minter.BlockProvision(params)
		amount := sdk.NewIntFromBigInt(mintedCoin.Amount.BigInt())
		// totalStakingSupply +  mintedCoin.Account > MaxTokenSupply
		if amount.Add(totalStakingSupply).GT(MaxTokenSupply) {
			mintedCoin.Amount = MaxTokenSupply.Sub(totalStakingSupply)
		}
		mintedCoins := sdk.NewCoins(mintedCoin)

		err := k.MintCoins(ctx, mintedCoins)
		if err != nil {
			panic(err)
		}

		// send the minted coins to the fee collector account
		err = k.AddCollectedFees(ctx, mintedCoins)
		if err != nil {
			panic(err)
		}

		if mintedCoin.Amount.IsInt64() {
			defer telemetry.ModuleSetGauge(types.ModuleName, float32(mintedCoin.Amount.Int64()), "minted_tokens")
		}

		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeMint,
				sdk.NewAttribute(types.AttributeKeyBondedRatio, bondedRatio.String()),
				sdk.NewAttribute(types.AttributeKeyInflation, minter.Inflation.String()),
				sdk.NewAttribute(types.AttributeKeyAnnualProvisions, minter.AnnualProvisions.String()),
				sdk.NewAttribute(sdk.AttributeKeyAmount, mintedCoin.Amount.String()),
			),
		)
	} else {
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeMint,
				sdk.NewAttribute("Message", "token aleady has max supply"),
				sdk.NewAttribute("MaxToeknSupply", MaxTokenSupply.String()),
			),
		)
	}

}
