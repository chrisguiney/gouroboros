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
	"math/big"
	"testing"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/stretchr/testify/require"
)

func exUnitPrices(memNum, memDen, stepNum, stepDen int64) common.ExUnitPrice {
	return common.ExUnitPrice{
		MemPrice:  &cbor.Rat{Rat: big.NewRat(memNum, memDen)},
		StepPrice: &cbor.Rat{Rat: big.NewRat(stepNum, stepDen)},
	}
}

// txscriptfee rounds up once over the sum of both components.
func TestExecutionUnitsFee(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		prices common.ExUnitPrice
		units  []common.ExUnits
		want   uint64
	}{
		"single redeemer": {
			prices: exUnitPrices(577, 10000, 721, 10000000),
			units:  []common.ExUnits{{Memory: 14_000_000, Steps: 10_000_000_000}},
			// 14e6*0.0577 + 1e10*0.0000721 = 807800 + 721000
			want: 1_528_800,
		},
		"one ceiling over the sum": {
			prices: exUnitPrices(1, 2, 1, 2),
			units:  []common.ExUnits{{Memory: 1, Steps: 0}, {Memory: 0, Steps: 1}},
			// 1/2 + 1/2 = 1, not ceil(1/2) + ceil(1/2) = 2
			want: 1,
		},
		"rounds up": {
			prices: exUnitPrices(1, 3, 0, 1),
			units:  []common.ExUnits{{Memory: 1, Steps: 0}},
			want:   1,
		},
		"no units": {
			prices: exUnitPrices(1, 2, 1, 2),
			want:   0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := common.ExecutionUnitsFee(tc.prices, tc.units)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestExecutionUnitsFeeRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	units := []common.ExUnits{{Memory: 1, Steps: 1}}
	_, err := common.ExecutionUnitsFee(common.ExUnitPrice{}, units)
	require.Error(t, err)
	_, err = common.ExecutionUnitsFee(exUnitPrices(-1, 2, 1, 2), units)
	require.Error(t, err)
	_, err = common.ExecutionUnitsFee(exUnitPrices(1, 2, 1, 2), []common.ExUnits{{Memory: -1}})
	require.Error(t, err)
}
