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
	"context"
	"hash/maphash"
	"testing"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
	"github.com/stretchr/testify/require"
)

func pureTestCostParams(language lang.LanguageVersion) []int64 {
	params := make([]int64, len(lang.GetParamNamesForVersion(language)))
	for i := range params {
		params[i] = int64(i + 1)
	}
	return params
}

func TestEvaluatePlutusScriptPureMatchesPooledPath(t *testing.T) {
	t.Parallel()
	script := buildMachinePoolTestV3Script(t)
	params := pureTestCostParams(lang.LanguageVersionV3)
	evalContext, err := cek.NewEvalContext(
		lang.LanguageVersionV3,
		cek.ProtoVersion{Major: 10},
		params,
	)
	require.NoError(t, err)
	want, err := script.Evaluate(
		machinePoolTestScriptContext(),
		machinePoolTestBudget(),
		evalContext,
	)
	require.NoError(t, err)
	require.NotZero(t, want.Steps)

	got, err := EvaluatePlutusScriptPure(
		context.Background(),
		PlutusScriptEvalInput{
			Language:      lang.LanguageVersionV3,
			Script:        script,
			ScriptContext: machinePoolTestScriptContext(),
			Budget:        machinePoolTestBudget(),
		},
		10,
		params,
	)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestEvaluatePlutusScriptPureReportsNonUnitResult(t *testing.T) {
	t.Parallel()
	script := buildMachinePoolTestV3ScriptReturning(t, &syn.Bool{Inner: true})
	_, err := EvaluatePlutusScriptPure(
		context.Background(),
		PlutusScriptEvalInput{
			Language:      lang.LanguageVersionV3,
			Script:        script,
			ScriptContext: machinePoolTestScriptContext(),
			Budget:        machinePoolTestBudget(),
		},
		10,
		pureTestCostParams(lang.LanguageVersionV3),
	)
	require.ErrorAs(t, err, new(PlutusScriptResultNotUnitError))
}

func TestEvaluatePlutusScriptPureBypassesProgramCache(t *testing.T) {
	t.Parallel()
	script := programCacheTestScript(t, lang.LanguageVersion{1, 0, 0}, 930001)
	inner, err := decodePlutusScript(script, true)
	require.NoError(t, err)
	_, err = EvaluatePlutusScriptPure(
		context.Background(),
		PlutusScriptEvalInput{
			Language:      lang.LanguageVersionV3,
			Script:        script,
			ScriptContext: machinePoolTestScriptContext(),
			Budget:        machinePoolTestBudget(),
		},
		10,
		pureTestCostParams(lang.LanguageVersionV3),
	)
	require.NoError(t, err)
	key := programCacheKey{
		hash:           maphash.Bytes(programCacheSeed, inner),
		ledgerLanguage: lang.LanguageVersionV3,
		protoMajor:     10,
	}
	_, cached := defaultProgramCache.get(key, inner)
	require.False(t, cached, "pure evaluation must not populate the cache")
}
