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
	"math/big"

	"github.com/blinklabs-io/gouroboros/ledger/common"
)

// PraosTipView holds the fields of a chain tip that Praos chain selection
// reads. VRFOutput is the tip block's leader VRF output; empty means absent.
type PraosTipView struct {
	BlockNumber          uint64
	Slot                 uint64
	IssuerVkey           common.IssuerVkey
	OpCertSequenceNumber uint64
	VRFOutput            []byte
}

// ComparePraosTips orders two tip views by the Praos chain-selection rules,
// without the era-specific switching restriction. It returns a positive value
// when a is preferred, a negative value when b is, and zero for a tie.
//
// The rules apply in order: higher block number; then, for tips from the same
// issuer in the same slot, the higher operational certificate issue number, so
// a replacement hot key beats a compromised one; then the lower leader VRF
// output, with an absent output ranking below any present one.
func ComparePraosTips(a, b PraosTipView) int {
	if a.BlockNumber != b.BlockNumber {
		if a.BlockNumber > b.BlockNumber {
			return 1
		}
		return -1
	}

	// Other issuers' counters are unrelated, so only the same issuer in the
	// same slot is ordered by issue number.
	if a.Slot == b.Slot && a.IssuerVkey == b.IssuerVkey {
		if a.OpCertSequenceNumber > b.OpCertSequenceNumber {
			return 1
		}
		if a.OpCertSequenceNumber < b.OpCertSequenceNumber {
			return -1
		}
	}

	if len(a.VRFOutput) == 0 && len(b.VRFOutput) == 0 {
		return 0
	}
	if len(a.VRFOutput) == 0 {
		return -1
	}
	if len(b.VRFOutput) == 0 {
		return 1
	}

	// Lower VRF output is preferred.
	aVRF := new(big.Int).SetBytes(a.VRFOutput)
	bVRF := new(big.Int).SetBytes(b.VRFOutput)
	switch cmp := aVRF.Cmp(bVRF); {
	case cmp < 0:
		return 1
	case cmp > 0:
		return -1
	}
	return 0
}

// PreferPraosCandidate reports whether candidate should replace current. A tie
// keeps the current chain. From the Conway protocol major version, a candidate
// at the same block number whose slot is more than five slots from the current
// tip's is not taken, which is the restricted VRF tiebreaker.
func PreferPraosCandidate(
	current, candidate PraosTipView,
	protocolMajor uint,
) bool {
	if protocolMajor >= common.ProtocolVersionConway &&
		current.BlockNumber == candidate.BlockNumber {
		earlier, later := current.Slot, candidate.Slot
		if earlier > later {
			earlier, later = later, earlier
		}
		if later-earlier > conwayVRFTiebreakerMaxDistance {
			return false
		}
	}
	return ComparePraosTips(candidate, current) > 0
}

func praosTipView(tip ChainTip) PraosTipView {
	return PraosTipView{
		BlockNumber:          tip.BlockNumber(),
		Slot:                 tip.Slot(),
		IssuerVkey:           tip.IssuerVkey(),
		OpCertSequenceNumber: tip.OpCertSequenceNumber(),
		VRFOutput:            tip.VRFOutput(),
	}
}
