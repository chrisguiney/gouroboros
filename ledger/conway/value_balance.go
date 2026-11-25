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

package conway

import (
	"bytes"
	"math/big"
	"slices"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// ValueBalanceAsset is one asset quantity in a ValueBalance.
type ValueBalanceAsset struct {
	Policy   common.Blake2b224
	Name     []byte
	Quantity *big.Int
}

// ValueBalance is the value a transaction consumes and produces.
type ValueBalance struct {
	ConsumedCoin   *big.Int
	ConsumedAssets []ValueBalanceAsset
	ProducedCoin   *big.Int
	ProducedAssets []ValueBalanceAsset
	// Deposits and Refunds are already part of ProducedCoin and ConsumedCoin.
	Deposits *big.Int
	Refunds  *big.Int
}

type valueBalanceKey struct {
	policy common.Blake2b224
	name   string
}

type valueBalance struct {
	consumedCoin   *big.Int
	producedCoin   *big.Int
	consumedAssets map[valueBalanceKey]*big.Int
	producedAssets map[valueBalanceKey]*big.Int
	deposits       *big.Int
	refunds        *big.Int
}

func addAsset(m map[valueBalanceKey]*big.Int, key valueBalanceKey, q *big.Int) {
	if m[key] == nil {
		m[key] = new(big.Int)
	}
	m[key].Add(m[key], q)
}

// computeValueBalance follows getConsumedMaryValue and conwayProducedValue:
// consumed is the inputs (an unresolved input counts nothing), withdrawals,
// certificate refunds and the positively minted assets; produced is the
// outputs, fee, certificate, pool and proposal deposits, treasury donation
// and the burned assets. A minted or burned lovelace amount moves the coin
// totals and is not an asset.
func computeValueBalance(
	tx common.Transaction,
	slot uint64,
	ls common.LedgerState,
	pp *ConwayProtocolParameters,
) (*valueBalance, error) {
	b := &valueBalance{
		consumedCoin:   new(big.Int),
		producedCoin:   new(big.Int),
		consumedAssets: make(map[valueBalanceKey]*big.Int),
		producedAssets: make(map[valueBalanceKey]*big.Int),
	}
	for _, tmpInput := range tx.Inputs() {
		tmpUtxo, err := ls.UtxoById(tmpInput)
		// Ignore errors fetching the UTxO and exclude it from calculations
		if err != nil {
			continue
		}
		if amount := tmpUtxo.Output.Amount(); amount != nil {
			b.consumedCoin.Add(b.consumedCoin, amount)
		}
		if assets := tmpUtxo.Output.Assets(); assets != nil {
			assets.RangeUnordered(func(
				policy common.Blake2b224,
				assetName []byte,
				amount common.MultiAssetTypeOutput,
			) bool {
				if amount != nil {
					addAsset(b.consumedAssets, valueBalanceKey{policy, string(assetName)}, amount)
				}
				return true
			})
		}
	}
	for _, tmpWithdrawalAmount := range tx.Withdrawals() {
		if tmpWithdrawalAmount != nil {
			b.consumedCoin.Add(b.consumedCoin, tmpWithdrawalAmount)
		}
	}
	certificateRefunds, certificateDeposits, err := conwayValueConservationDeposits(
		tx,
		ls,
		pp,
	)
	if err != nil {
		return nil, err
	}
	b.refunds = new(big.Int).Set(certificateRefunds)
	b.deposits = new(big.Int).Set(certificateDeposits)
	b.consumedCoin.Add(b.consumedCoin, certificateRefunds)
	if mint := tx.AssetMint(); mint != nil {
		mint.RangeUnordered(func(
			policy common.Blake2b224,
			assetName []byte,
			amount common.MultiAssetTypeMint,
		) bool {
			if amount == nil {
				return true
			}
			if policy == (common.Blake2b224{}) && len(assetName) == 0 {
				b.consumedCoin.Add(b.consumedCoin, amount)
				return true
			}
			key := valueBalanceKey{policy, string(assetName)}
			if amount.Sign() >= 0 {
				addAsset(b.consumedAssets, key, amount)
			} else {
				addAsset(b.producedAssets, key, new(big.Int).Neg(amount))
			}
			return true
		})
	}
	for _, tmpOutput := range tx.Outputs() {
		if amount := tmpOutput.Amount(); amount != nil {
			b.producedCoin.Add(b.producedCoin, amount)
		}
		if assets := tmpOutput.Assets(); assets != nil {
			assets.RangeUnordered(func(
				policy common.Blake2b224,
				assetName []byte,
				amount common.MultiAssetTypeOutput,
			) bool {
				if amount != nil {
					addAsset(b.producedAssets, valueBalanceKey{policy, string(assetName)}, amount)
				}
				return true
			})
		}
	}
	if fee := tx.Fee(); fee != nil {
		b.producedCoin.Add(b.producedCoin, fee)
	}
	b.producedCoin.Add(b.producedCoin, certificateDeposits)
	seenPoolRegistrations := make(map[common.PoolKeyHash]struct{})
	for _, cert := range tx.Certificates() {
		switch tmpCert := cert.(type) {
		case *common.PoolRegistrationCertificate:
			operator := common.Blake2b224(tmpCert.Operator)
			if _, seen := seenPoolRegistrations[operator]; seen {
				continue
			}
			seenPoolRegistrations[operator] = struct{}{}
			depositDue, err := common.PoolRegistrationDepositDue(
				ls, slot, operator,
			)
			if err != nil {
				return nil, err
			}
			if depositDue {
				poolDeposit := new(big.Int).SetUint64(uint64(pp.PoolDeposit))
				b.producedCoin.Add(b.producedCoin, poolDeposit)
				b.deposits.Add(b.deposits, poolDeposit)
			}
		}
	}
	for range tx.ProposalProcedures() {
		proposalDeposit := new(big.Int).SetUint64(uint64(pp.GovActionDeposit))
		b.producedCoin.Add(b.producedCoin, proposalDeposit)
		b.deposits.Add(b.deposits, proposalDeposit)
	}
	// Treasury donation: value leaving the transaction to the treasury.
	donation := tx.Donation()
	if donation != nil && donation.Sign() > 0 {
		if err := ValidateTreasuryDonationScriptCompatibility(tx, ls); err != nil {
			return nil, err
		}
		b.producedCoin.Add(b.producedCoin, donation)
	}
	return b, nil
}

func sortedAssets(m map[valueBalanceKey]*big.Int) []ValueBalanceAsset {
	out := make([]ValueBalanceAsset, 0, len(m))
	for key, q := range m {
		if q.Sign() == 0 {
			continue
		}
		out = append(out, ValueBalanceAsset{
			Policy:   key.policy,
			Name:     []byte(key.name),
			Quantity: new(big.Int).Set(q),
		})
	}
	slices.SortFunc(out, func(a, b ValueBalanceAsset) int {
		if c := bytes.Compare(a.Policy[:], b.Policy[:]); c != 0 {
			return c
		}
		return bytes.Compare(a.Name, b.Name)
	})
	return out
}

// ComputeValueBalance returns the consumed and produced value of tx against
// ls, without comparing them. Assets are sorted by policy and then name, and
// zero quantities are omitted.
func ComputeValueBalance(
	tx common.Transaction,
	slot uint64,
	ls common.LedgerState,
	pp *ConwayProtocolParameters,
) (ValueBalance, error) {
	b, err := computeValueBalance(tx, slot, ls, pp)
	if err != nil {
		return ValueBalance{}, err
	}
	return ValueBalance{
		ConsumedCoin:   b.consumedCoin,
		ConsumedAssets: sortedAssets(b.consumedAssets),
		ProducedCoin:   b.producedCoin,
		ProducedAssets: sortedAssets(b.producedAssets),
		Deposits:       b.deposits,
		Refunds:        b.refunds,
	}, nil
}
