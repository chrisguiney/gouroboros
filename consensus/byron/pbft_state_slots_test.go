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

package byron

import (
	"testing"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/stretchr/testify/require"
)

func slotIssuer(b byte) common.Blake2b224 {
	var h common.Blake2b224
	h[0] = b
	return h
}

func signers(entries ...[2]uint64) []PBFTSigner {
	out := make([]PBFTSigner, 0, len(entries))
	for _, e := range entries {
		out = append(out, PBFTSigner{
			Issuer: slotIssuer(byte(e[0])), Slot: e[1], SlotKnown: true,
		})
	}
	return out
}

// TestPBFTTransitionAtSlotOrder pins the reference's slot rule: a signed slot
// may equal the last signer's (a boundary block shares its epoch's first
// slot) but not precede it.
func TestPBFTTransitionAtSlotOrder(t *testing.T) {
	t.Parallel()
	state, err := NewPBFTStateWithSlots(signers([2]uint64{1, 10}, [2]uint64{2, 20}), 5, 1, 1)
	require.NoError(t, err)

	_, err = state.TransitionAt(slotIssuer(3), 19)
	require.ErrorIs(t, err, ErrPBFTInvalidSlot)
	_, err = state.TransitionAt(slotIssuer(3), 20)
	require.NoError(t, err)
	_, err = state.TransitionAt(slotIssuer(3), 21)
	require.NoError(t, err)

	// A window with no signers is genesis: any slot is acceptable.
	empty, err := NewPBFTStateWithSlots(nil, 5, 1, 1)
	require.NoError(t, err)
	_, err = empty.TransitionAt(slotIssuer(1), 0)
	require.NoError(t, err)
}

// TestPBFTTransitionAtWindowSlides pins that the signature that leaves the
// window no longer counts: the same issuer is over the limit when the oldest
// entry is someone else's, and within it when the oldest entry is its own.
func TestPBFTTransitionAtWindowSlides(t *testing.T) {
	t.Parallel()
	// k = 5, threshold 2/5: at most two signatures per issuer in the window.
	inside, err := NewPBFTStateWithSlots(
		signers([2]uint64{1, 1}, [2]uint64{1, 2}, [2]uint64{2, 3}, [2]uint64{3, 4}, [2]uint64{4, 5}), 5, 2, 5)
	require.NoError(t, err)
	next, err := inside.TransitionAt(slotIssuer(1), 6)
	require.NoError(t, err, "the oldest signature by the same issuer leaves the window")
	require.Equal(t, signers(
		[2]uint64{1, 2}, [2]uint64{2, 3}, [2]uint64{3, 4}, [2]uint64{4, 5}, [2]uint64{1, 6}), next.Signers())

	over, err := NewPBFTStateWithSlots(
		signers([2]uint64{2, 1}, [2]uint64{1, 2}, [2]uint64{1, 3}, [2]uint64{3, 4}, [2]uint64{4, 5}), 5, 2, 5)
	require.NoError(t, err)
	_, err = over.TransitionAt(slotIssuer(1), 6)
	require.ErrorContains(t, err, "threshold exceeded")
}

// TestPBFTSignersWithoutSlots pins that the constructors and transitions that
// carry no slot keep working, report their signers without a slot, and leave
// the slot rule unenforced against them.
func TestPBFTSignersWithoutSlots(t *testing.T) {
	t.Parallel()
	state, err := NewPBFTState([]common.Blake2b224{slotIssuer(1)}, 5)
	require.NoError(t, err)
	require.Equal(t, []PBFTSigner{{Issuer: slotIssuer(1)}}, state.Signers())
	next, err := state.Transition(slotIssuer(2))
	require.NoError(t, err)
	require.False(t, next.Signers()[1].SlotKnown)
	_, err = next.TransitionAt(slotIssuer(3), 0)
	require.NoError(t, err)
	require.Equal(t, []common.Blake2b224{slotIssuer(1), slotIssuer(2)}, next.SignatureHistory())
}

// TestPBFTSlotsFollowBranches pins that extending an older state, as a
// rollback does, keeps each branch's own slots.
func TestPBFTSlotsFollowBranches(t *testing.T) {
	t.Parallel()
	base, err := NewPBFTStateWithSlots(signers([2]uint64{1, 1}), 5, 1, 1)
	require.NoError(t, err)
	a, err := base.TransitionAt(slotIssuer(2), 2)
	require.NoError(t, err)
	b, err := base.TransitionAt(slotIssuer(3), 3)
	require.NoError(t, err)
	require.Equal(t, signers([2]uint64{1, 1}, [2]uint64{2, 2}), a.Signers())
	require.Equal(t, signers([2]uint64{1, 1}, [2]uint64{3, 3}), b.Signers())
	require.Equal(t, signers([2]uint64{1, 1}), base.Signers())
}

func TestNewPBFTStateWithSlotsRejectsOversizedHistory(t *testing.T) {
	t.Parallel()
	_, err := NewPBFTStateWithSlots(signers([2]uint64{1, 1}, [2]uint64{2, 2}), 1, 1, 1)
	require.Error(t, err)
}
