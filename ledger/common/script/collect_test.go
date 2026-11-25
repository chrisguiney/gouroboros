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

package script_test

import (
	"math/big"
	"slices"
	"testing"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/common/script"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
	mockledger "github.com/blinklabs-io/ouroboros-mock/ledger"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
	"github.com/stretchr/testify/require"
)

// collectV3Script is a PlutusV3 script that returns unit, or fails.
func collectV3Script(t *testing.T, fail bool, salt int64) common.PlutusV3Script {
	t.Helper()
	// The salt makes each script's hash its own.
	var body syn.Term[syn.DeBruijn] = &syn.Apply[syn.DeBruijn]{
		Function: &syn.Lambda[syn.DeBruijn]{Body: &syn.Constant{Con: &syn.Unit{}}},
		Argument: &syn.Constant{Con: &syn.Integer{Inner: big.NewInt(salt)}},
	}
	if fail {
		body = &syn.Error{}
	}
	flat, err := syn.Encode(&syn.Program[syn.DeBruijn]{
		Version: [3]uint32{1, 1, 0},
		Term:    &syn.Lambda[syn.DeBruijn]{Body: body},
	})
	require.NoError(t, err)
	wrapper, err := cbor.Encode(flat)
	require.NoError(t, err)
	return common.PlutusV3Script(wrapper)
}

// collectAll is the scripts of every redeemer that has one, or the first
// error collecting a redeemer's script: what a caller that stops at the first
// failure sees.
func collectAll(
	tx common.Transaction,
	ls common.LedgerState,
	protocolMajor uint,
	maxPlutusVersion uint,
	era string,
) ([]script.CollectedScript, error) {
	results, err := script.CollectPlutusScripts(
		tx, ls, protocolMajor, maxPlutusVersion, era,
	)
	if err != nil {
		return nil, err
	}
	var out []script.CollectedScript
	for _, r := range results {
		if r.Err != nil {
			return nil, r.Err
		}
		if r.Script != nil {
			out = append(out, *r.Script)
		}
	}
	return out, nil
}

// collectMintTx mints one asset under each script's policy, with a redeemer
// per policy declaring distinct units. The redeemer of index i points at the
// i-th policy in hash order.
func collectMintTx(
	t *testing.T,
	scripts ...common.PlutusV3Script,
) *conway.ConwayTransaction {
	t.Helper()
	policies := map[common.Blake2b224]map[cbor.ByteString]*big.Int{}
	redeemers := map[common.RedeemerKey]common.RedeemerValue{}
	for _, s := range scripts {
		policies[s.Hash()] = map[cbor.ByteString]*big.Int{
			cbor.NewByteString(nil): big.NewInt(1),
		}
	}
	for i := range scripts {
		redeemers[common.RedeemerKey{Tag: common.RedeemerTagMint, Index: uint32(i)}] = common.RedeemerValue{
			Data:    common.Datum{Data: data.NewInteger(big.NewInt(int64(i)))},
			ExUnits: common.ExUnits{Memory: int64(100 + i), Steps: int64(200 + i)},
		}
	}
	mint := common.NewMultiAsset(policies)
	return &conway.ConwayTransaction{
		Body: conway.ConwayTransactionBody{TxMint: &mint},
		WitnessSet: conway.ConwayTransactionWitnessSet{
			WsPlutusV3Scripts: cbor.NewSetType(scripts, false),
			WsRedeemers:       conway.ConwayRedeemers{Redeemers: redeemers},
		},
		TxIsValid: true,
	}
}

// policyOrder returns the scripts in policy hash order, which is the order
// the mint redeemer indices count.
func policyOrder(scripts ...common.PlutusV3Script) []common.PlutusV3Script {
	out := slices.Clone(scripts)
	slices.SortFunc(out, func(a, b common.PlutusV3Script) int {
		ha, hb := a.Hash(), b.Hash()
		return slices.Compare(ha[:], hb[:])
	})
	return out
}

func TestCollectPlutusScriptsReturnsEveryRedeemersScript(t *testing.T) {
	t.Parallel()
	good, bad := collectV3Script(t, false, 1), collectV3Script(t, true, 2)
	tx := collectMintTx(t, good, bad)
	ordered := policyOrder(good, bad)
	got, err := collectAll(
		tx, mockledger.NewLedgerStateBuilder().Build(), 9, 3, "Conway",
	)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for i, c := range got {
		require.Equal(t, common.RedeemerKey{Tag: common.RedeemerTagMint, Index: uint32(i)}, c.Key)
		require.Equal(t, ordered[i].Hash(), c.ScriptHash)
		require.Equal(t, uint(3), c.Language())
		require.Equal(t, common.ExUnits{Memory: int64(100 + i), Steps: int64(200 + i)}, c.ExUnits, "the declared units")
		require.IsType(t, script.ScriptPurposeMinting{}, c.Purpose)
		require.Nil(t, c.Datum, "a V3 script has no separate datum")
		require.Nil(t, c.Redeemer, "a V3 script's redeemer is in its context")
		require.NotNil(t, c.Context)
	}
}

func TestCollectPlutusScriptsSkipsARedeemerWhoseScriptIsAbsent(t *testing.T) {
	t.Parallel()
	good := collectV3Script(t, false, 1)
	tx := collectMintTx(t, good)
	tx.WitnessSet.WsPlutusV3Scripts = cbor.SetType[common.PlutusV3Script]{}
	got, err := collectAll(
		tx, mockledger.NewLedgerStateBuilder().Build(), 9, 3, "Conway",
	)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestCollectPlutusScriptsReportsAnExtraRedeemer(t *testing.T) {
	t.Parallel()
	good := collectV3Script(t, false, 1)
	tx := collectMintTx(t, good)
	extra := common.RedeemerKey{Tag: common.RedeemerTagSpend, Index: 3}
	tx.WitnessSet.WsRedeemers.Redeemers[extra] = common.RedeemerValue{Data: common.Datum{Data: data.NewInteger(big.NewInt(0))}}
	_, err := collectAll(
		tx, mockledger.NewLedgerStateBuilder().Build(), 9, 3, "Conway",
	)
	require.ErrorIs(t, err, common.ExtraRedeemerError{RedeemerKey: extra})
}

func TestCollectPlutusScriptsRefusesALanguageTheEraLacks(t *testing.T) {
	t.Parallel()
	tx := collectMintTx(t, collectV3Script(t, false, 1))
	_, err := collectAll(
		tx, mockledger.NewLedgerStateBuilder().Build(), 8, 2, "Babbage",
	)
	require.ErrorIs(t, err, common.PlutusScriptValidationUnsupportedError{Era: "Babbage"})
}

// Each collected script runs on its own: one script's failure does not stop
// the others from reporting their units, which the redeemer-order evaluation
// of EvaluatePlutusScriptsPure cannot show.
func TestEvaluateCollectedScriptPureRunsEachScriptIndependently(t *testing.T) {
	t.Parallel()
	good, bad := collectV3Script(t, false, 1), collectV3Script(t, true, 2)
	tx := collectMintTx(t, good, bad)
	ls := mockledger.NewLedgerStateBuilder().Build()
	budget := &common.ExUnits{Memory: 10_000_000, Steps: 10_000_000}
	collected, err := collectAll(tx, ls, 9, 3, "Conway")
	require.NoError(t, err)
	require.Len(t, collected, 2)
	models := map[uint][]int64{2: v3CostModel(t)}
	var failed, ok int
	for _, c := range collected {
		units, err := script.EvaluateCollectedScriptPure(c, 9, models, budget)
		if err != nil {
			var scriptErr common.PlutusScriptFailedError
			require.ErrorAs(t, err, &scriptErr)
			require.Equal(t, c.Key.Index, scriptErr.Index)
			require.Equal(t, c.ScriptHash, scriptErr.ScriptHash)
			failed++
			continue
		}
		require.Positive(t, units.Memory)
		require.Positive(t, units.Steps)
		ok++
	}
	require.Equal(t, 1, failed)
	require.Equal(t, 1, ok)
	// The whole-transaction evaluation stops at the failure.
	_, err = script.EvaluatePlutusScriptsPure(tx, ls, 9, models, 3, "Conway", budget)
	var scriptErr common.PlutusScriptFailedError
	require.ErrorAs(t, err, &scriptErr)
}

// EvaluatePlutusScripts is the collector and the runner in redeemer order.
func TestEvaluatePlutusScriptsAgreesWithCollectThenExecute(t *testing.T) {
	t.Parallel()
	good := collectV3Script(t, false, 1)
	other := collectV3Script(t, false, 2)
	tx := collectMintTx(t, good, other)
	ls := mockledger.NewLedgerStateBuilder().Build()
	budget := &common.ExUnits{Memory: 10_000_000, Steps: 10_000_000}
	models := map[uint][]int64{2: v3CostModel(t)}
	want, err := script.EvaluatePlutusScriptsPure(tx, ls, 9, models, 3, "Conway", budget)
	require.NoError(t, err)
	collected, err := collectAll(tx, ls, 9, 3, "Conway")
	require.NoError(t, err)
	got := map[common.RedeemerKey]common.ExUnits{}
	for _, c := range collected {
		units, err := script.EvaluateCollectedScriptPure(c, 9, models, budget)
		require.NoError(t, err)
		got[c.Key] = units
	}
	require.Equal(t, want, got)
}

// v3CostModel is a PlutusV3 cost model with every parameter 1.
func v3CostModel(t *testing.T) []int64 {
	t.Helper()
	params := make([]int64, len(lang.CostModelParamNamesV3))
	for i := range params {
		params[i] = 1
	}
	return params
}

// Every redeemer is reported on its own: a collected script, a redeemer whose
// script is absent, and a redeemer with no purpose each have a result, in
// redeemer order.
func TestCollectPlutusScriptsReportsEachRedeemerOnItsOwn(t *testing.T) {
	t.Parallel()
	present, absent := collectV3Script(t, false, 1), collectV3Script(t, false, 2)
	tx := collectMintTx(t, present, absent)
	tx.WitnessSet.WsPlutusV3Scripts = cbor.NewSetType([]common.PlutusV3Script{present}, false)
	got, err := script.CollectPlutusScripts(
		tx, mockledger.NewLedgerStateBuilder().Build(), 9, 3, "Conway",
	)
	require.NoError(t, err)
	require.Len(t, got, 2)
	ordered := policyOrder(present, absent)
	for i, r := range got {
		require.Equal(t, common.RedeemerKey{Tag: common.RedeemerTagMint, Index: uint32(i)}, r.Key)
		require.NoError(t, r.Err)
		if ordered[i].Hash() == present.Hash() {
			require.NotNil(t, r.Script, "the witnessed script is collected")
		} else {
			require.Nil(t, r.Script, "an absent script is skipped")
		}
	}
}
