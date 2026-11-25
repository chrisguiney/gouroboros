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

package common_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger/allegra"
	"github.com/blinklabs-io/gouroboros/ledger/alonzo"
	"github.com/blinklabs-io/gouroboros/ledger/babbage"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
	"github.com/blinklabs-io/gouroboros/ledger/dijkstra"
	"github.com/blinklabs-io/gouroboros/ledger/mary"
	"github.com/blinklabs-io/gouroboros/ledger/shelley"
	mockledger "github.com/blinklabs-io/ouroboros-mock/ledger"
	"github.com/stretchr/testify/require"
)

func TestUtxoValidationRuleFamilyCoversEveryEraDescriptor(t *testing.T) {
	t.Parallel()
	eras := map[string][]common.UtxoValidationRuleDescriptor{
		"shelley":  shelley.UtxoValidationRuleDescriptors(),
		"allegra":  allegra.UtxoValidationRuleDescriptors(),
		"mary":     mary.UtxoValidationRuleDescriptors(),
		"alonzo":   alonzo.UtxoValidationRuleDescriptors(),
		"babbage":  babbage.UtxoValidationRuleDescriptors(),
		"conway":   conway.UtxoValidationRuleDescriptors(),
		"dijkstra": dijkstra.UtxoValidationRuleDescriptors(),
	}
	for era, descriptors := range eras {
		for _, d := range descriptors {
			_, ok := common.UtxoValidationRuleFamily(d.Id)
			require.Truef(t, ok, "%s rule %q has no STS family", era, d.Id)
		}
	}
}

func TestUtxoValidationRuleFamilySamples(t *testing.T) {
	t.Parallel()
	want := map[common.UtxoValidationRuleId]common.UtxoValidationFamily{
		common.UtxoValidationRuleFeeTooSmall:              "UTXO",
		common.UtxoValidationRuleRequiredVKeyWitnesses:    "UTXOW",
		common.UtxoValidationRulePlutusScripts:            "UTXOS",
		common.UtxoValidationRuleDelegation:               "DELEG",
		common.UtxoValidationRulePoolCertificates:         "POOL",
		common.UtxoValidationRuleCommitteeCertificates:    "GOVCERT",
		common.UtxoValidationRuleProposalDeposit:          "GOV",
		common.UtxoValidationRuleWithdrawals:              "CERTS",
		common.UtxoValidationRuleProtocolParameterUpdates: "PPUP",
		common.UtxoValidationRuleRefScriptSizePerTx:       "LEDGER",
		// Conway reports the treasury mismatch as ConwayTreasuryValueMismatch
		// of the LEDGER rule (Rules/Ledger.hs), not of UTXO.
		common.UtxoValidationRuleCurrentTreasuryValue: "LEDGER",
	}
	for id, family := range want {
		got, ok := common.UtxoValidationRuleFamily(id)
		require.True(t, ok, id)
		require.Equal(t, family, got, id)
	}
	_, ok := common.UtxoValidationRuleFamily("no-such-rule")
	require.False(t, ok)
}

func TestCollectUtxoValidationFailuresReportsEveryFailure(t *testing.T) {
	t.Parallel()
	errA := errors.New("a")
	errC := errors.New("c")
	fail := func(err error) common.UtxoValidationRuleFunc {
		return func(
			common.Transaction, uint64, common.LedgerState,
			common.ProtocolParameters,
		) error {
			return err
		}
	}
	rules := []common.UtxoValidationRuleFunc{
		fail(errA), fail(nil), fail(errC),
	}
	descriptors := []common.UtxoValidationRuleDescriptor{
		{Id: common.UtxoValidationRuleFeeTooSmall},
		{Id: common.UtxoValidationRuleBadInputs},
		{Id: common.UtxoValidationRuleDelegation},
	}
	all := common.CollectUtxoValidationFailures(
		nil, 0, nil, nil, rules, descriptors,
	)
	require.Len(t, all, 2)
	require.Equal(t, 0, all[0].RuleIndex)
	require.Equal(t, common.UtxoValidationRuleFeeTooSmall, all[0].RuleId)
	require.Equal(t, common.UtxoValidationFamily("UTXO"), all[0].Family)
	require.ErrorIs(t, all[0].Err, errA)
	require.Equal(t, 2, all[1].RuleIndex)
	require.ErrorIs(t, all[1].Err, errC)

	onlyDeleg := common.CollectUtxoValidationFailures(
		nil, 0, nil, nil, rules, descriptors, "DELEG",
	)
	require.Len(t, onlyDeleg, 1)
	require.Equal(t, 2, onlyDeleg[0].RuleIndex)

	mismatch := common.CollectUtxoValidationFailures(
		nil, 0, nil, nil, rules, descriptors[:2],
	)
	require.Len(t, mismatch, 1)
	require.Equal(t, -1, mismatch[0].RuleIndex)
}

func TestEraUtxoValidationFailuresMatchFirstFailure(t *testing.T) {
	t.Parallel()
	ls := mockledger.NewLedgerStateBuilder().Build()
	cases := map[string]struct {
		tx       common.Transaction
		pp       common.ProtocolParameters
		rules    []common.UtxoValidationRuleFunc
		failures func(
			common.Transaction, uint64, common.LedgerState,
			common.ProtocolParameters, ...common.UtxoValidationFamily,
		) []common.UtxoValidationFailure
	}{
		"shelley": {
			tx: &shelley.ShelleyTransaction{}, pp: &shelley.ShelleyProtocolParameters{},
			rules: shelley.UtxoValidationRules, failures: shelley.UtxoValidationFailures,
		},
		"allegra": {
			tx: &allegra.AllegraTransaction{}, pp: &allegra.AllegraProtocolParameters{},
			rules: allegra.UtxoValidationRules, failures: allegra.UtxoValidationFailures,
		},
		"mary": {
			tx: &mary.MaryTransaction{}, pp: &mary.MaryProtocolParameters{},
			rules: mary.UtxoValidationRules, failures: mary.UtxoValidationFailures,
		},
		"alonzo": {
			tx: &alonzo.AlonzoTransaction{}, pp: &alonzo.AlonzoProtocolParameters{},
			rules: alonzo.UtxoValidationRules, failures: alonzo.UtxoValidationFailures,
		},
		"babbage": {
			tx: &babbage.BabbageTransaction{}, pp: &babbage.BabbageProtocolParameters{},
			rules: babbage.UtxoValidationRules, failures: babbage.UtxoValidationFailures,
		},
		"conway": {
			tx: &conway.ConwayTransaction{}, pp: &conway.ConwayProtocolParameters{},
			rules: conway.UtxoValidationRules, failures: conway.UtxoValidationFailures,
		},
		"dijkstra": {
			tx: &dijkstra.DijkstraTransaction{}, pp: &dijkstra.DijkstraProtocolParameters{},
			rules: dijkstra.UtxoValidationRules, failures: dijkstra.UtxoValidationFailures,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			failures := tc.failures(tc.tx, 100, ls, tc.pp)
			single := common.VerifyTransaction(tc.tx, 100, ls, tc.pp, tc.rules)
			require.Error(t, single)
			require.Greater(t, len(failures), 1, "empty tx fails several rules")
			var verr *common.ValidationError
			require.ErrorAs(t, single, &verr)
			require.Equal(t, failures[0].RuleIndex, verr.Details["rule_index"])
			require.ErrorIs(t, single, failures[0].Err)
		})
	}
}

func TestResolvedUtxosFeedsConsumedReferenceScriptSize(t *testing.T) {
	t.Parallel()
	input := shelley.MustNewShelleyTransactionInput(
		strings.Repeat("01", 32), 0,
	)
	other := shelley.MustNewShelleyTransactionInput(
		strings.Repeat("02", 32), 0,
	)
	utxos := common.ResolvedUtxos{{
		Id: input,
		Output: &babbage.BabbageTransactionOutput{
			TxOutScriptRef: &common.ScriptRef{
				Script: make(common.PlutusV3Script, 60),
			},
		},
	}}
	body := &conway.ConwayTransactionBody{
		TxInputs: conway.NewConwayTransactionInputSet(
			[]shelley.ShelleyTransactionInput{input},
		),
	}
	size, err := common.ConsumedReferenceScriptSize(body, utxos)
	require.NoError(t, err)
	require.Equal(t, uint64(60), size)

	_, err = utxos.UtxoById(other)
	require.ErrorIs(t, err, common.ErrUtxoNotFound)
}
