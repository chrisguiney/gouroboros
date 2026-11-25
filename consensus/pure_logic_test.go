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

package consensus

import (
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/stretchr/testify/require"
)

func TestComparePraosTipsOrdering(t *testing.T) {
	t.Parallel()
	issuerA := common.IssuerVkey{1}
	issuerB := common.IssuerVkey{2}
	cases := []struct {
		name string
		a, b PraosTipView
		want int
	}{
		{"longer chain wins", PraosTipView{BlockNumber: 5}, PraosTipView{BlockNumber: 4}, 1},
		{"shorter chain loses", PraosTipView{BlockNumber: 4}, PraosTipView{BlockNumber: 5}, -1},
		{
			"lower vrf wins at equal height",
			PraosTipView{BlockNumber: 5, VRFOutput: []byte{1}},
			PraosTipView{BlockNumber: 5, VRFOutput: []byte{2}},
			1,
		},
		{
			"missing vrf loses",
			PraosTipView{BlockNumber: 5},
			PraosTipView{BlockNumber: 5, VRFOutput: []byte{2}},
			-1,
		},
		{"both missing vrf tie", PraosTipView{BlockNumber: 5}, PraosTipView{BlockNumber: 5}, 0},
		{
			"same issuer same slot higher opcert wins over lower vrf",
			PraosTipView{BlockNumber: 5, Slot: 9, IssuerVkey: issuerA, OpCertSequenceNumber: 2, VRFOutput: []byte{9}},
			PraosTipView{BlockNumber: 5, Slot: 9, IssuerVkey: issuerA, OpCertSequenceNumber: 1, VRFOutput: []byte{1}},
			1,
		},
		{
			"different issuer ignores opcert and uses vrf",
			PraosTipView{BlockNumber: 5, Slot: 9, IssuerVkey: issuerA, OpCertSequenceNumber: 2, VRFOutput: []byte{9}},
			PraosTipView{BlockNumber: 5, Slot: 9, IssuerVkey: issuerB, OpCertSequenceNumber: 1, VRFOutput: []byte{1}},
			-1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, ComparePraosTips(tc.a, tc.b))
		})
	}
}

func TestPreferPraosCandidateConwayWindow(t *testing.T) {
	t.Parallel()
	current := PraosTipView{BlockNumber: 5, Slot: 100, VRFOutput: []byte{9}}
	near := PraosTipView{BlockNumber: 5, Slot: 105, VRFOutput: []byte{1}}
	far := PraosTipView{BlockNumber: 5, Slot: 106, VRFOutput: []byte{1}}
	require.True(t, PreferPraosCandidate(current, near, common.ProtocolVersionConway))
	require.False(t, PreferPraosCandidate(current, far, common.ProtocolVersionConway))
	// Before Conway the slot distance does not restrict switching.
	require.True(t, PreferPraosCandidate(current, far, common.ProtocolVersionConway-1))
	// A tie keeps the current chain.
	require.False(t, PreferPraosCandidate(current, current, common.ProtocolVersionConway))
}

func TestPraosSelectorUsesPureComparison(t *testing.T) {
	t.Parallel()
	selector := NewPraosChainSelector(2160)
	a := NewSimpleChainTip(10, 5, []byte{1})
	b := NewSimpleChainTip(10, 5, []byte{2})
	view := func(tip ChainTip) PraosTipView {
		return PraosTipView{
			BlockNumber:          tip.BlockNumber(),
			Slot:                 tip.Slot(),
			IssuerVkey:           tip.IssuerVkey(),
			OpCertSequenceNumber: tip.OpCertSequenceNumber(),
			VRFOutput:            tip.VRFOutput(),
		}
	}
	require.Equal(t, ComparePraosTips(view(a), view(b)), selector.Compare(a, b))
}
