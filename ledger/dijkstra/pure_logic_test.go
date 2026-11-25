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

package dijkstra

import (
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/common/script"
	mockledger "github.com/blinklabs-io/ouroboros-mock/ledger"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/stretchr/testify/require"
)

func TestEvaluatePlutusScriptsPureMatchesPooledEvaluation(t *testing.T) {
	t.Parallel()
	pp := dijkstraGuardTestPParams()
	ls := mockledger.NewLedgerStateBuilder().Build()
	tx := evaluateTestMintTx(
		t,
		dijkstraGuardTestPlutus(t, lang.LanguageVersionV3, false),
	)
	budget := evaluateTestBudget
	want, err := script.EvaluatePlutusScripts(
		tx, ls, pp.ProtocolVersion.Major, pp.CostModels, 3, EraNameDijkstra,
		&budget,
	)
	require.NoError(t, err)
	require.NotEmpty(t, want)

	got, err := script.EvaluatePlutusScriptsPure(
		tx, ls, pp.ProtocolVersion.Major, pp.CostModels, 3, EraNameDijkstra,
		&budget,
	)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestEvaluatePlutusScriptsPureReportsScriptFailure(t *testing.T) {
	t.Parallel()
	pp := dijkstraGuardTestPParams()
	ls := mockledger.NewLedgerStateBuilder().Build()
	tx := evaluateTestMintTx(
		t,
		dijkstraGuardTestPlutus(t, lang.LanguageVersionV3, true),
	)
	budget := evaluateTestBudget
	_, err := script.EvaluatePlutusScriptsPure(
		tx, ls, pp.ProtocolVersion.Major, pp.CostModels, 3, EraNameDijkstra,
		&budget,
	)
	var failed common.PlutusScriptFailedError
	require.ErrorAs(t, err, &failed)
}
