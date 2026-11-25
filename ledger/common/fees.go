// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"errors"
	"fmt"
	"math/big"
)

// CalculateExecutionUnitsFee returns the rounded-up fee for all declared
// redeemer execution-unit budgets across the transaction and subtransactions.
func CalculateExecutionUnitsFee(
	tx Transaction,
	prices ExUnitPrice,
) (uint64, error) {
	witnessSets := append(
		[]TransactionWitnessSet{tx.Witnesses()},
		SubTransactionWitnessSetsFromTransaction(tx)...,
	)
	var units []ExUnits
	for _, witnesses := range witnessSets {
		if witnesses == nil || witnesses.Redeemers() == nil {
			continue
		}
		for _, redeemer := range witnesses.Redeemers().Iter() {
			units = append(units, redeemer.ExUnits)
		}
	}
	return ExecutionUnitsFee(prices, units)
}

// ExecutionUnitsFee prices a list of execution-unit budgets. The rational
// total across all entries is rounded up once, as txscriptfee does over the
// summed ExUnits. With no entries the fee is zero and the prices are not
// examined.
func ExecutionUnitsFee(prices ExUnitPrice, units []ExUnits) (uint64, error) {
	if len(units) == 0 {
		return 0, nil
	}
	if prices.MemPrice == nil || prices.MemPrice.Rat == nil {
		return 0, errors.New("invalid execution memory price")
	}
	if prices.StepPrice == nil || prices.StepPrice.Rat == nil {
		return 0, errors.New("invalid execution step price")
	}
	if prices.MemPrice.Sign() < 0 || prices.StepPrice.Sign() < 0 {
		return 0, errors.New("execution prices must not be negative")
	}
	total := new(big.Rat)
	for _, u := range units {
		if u.Memory < 0 || u.Steps < 0 {
			return 0, errors.New("execution units must not be negative")
		}
		memoryFee := new(big.Rat).Mul(
			new(big.Rat).SetInt64(u.Memory),
			prices.MemPrice.Rat,
		)
		stepsFee := new(big.Rat).Mul(
			new(big.Rat).SetInt64(u.Steps),
			prices.StepPrice.Rat,
		)
		total.Add(total, memoryFee)
		total.Add(total, stepsFee)
	}
	fee := new(big.Int).Quo(total.Num(), total.Denom())
	if new(big.Int).Mod(total.Num(), total.Denom()).Sign() != 0 {
		fee.Add(fee, big.NewInt(1))
	}
	if !fee.IsUint64() {
		return 0, fmt.Errorf("execution fee overflow: %s", fee)
	}
	return fee.Uint64(), nil
}
