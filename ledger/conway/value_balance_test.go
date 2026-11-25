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

package conway_test

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/blinklabs-io/gouroboros/cbor"
	"github.com/blinklabs-io/gouroboros/ledger/babbage"
	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
	mockledger "github.com/blinklabs-io/ouroboros-mock/ledger"
	"github.com/stretchr/testify/require"
)

func vbCbor(t *testing.T, v any) []byte {
	t.Helper()
	b, err := cbor.Encode(v)
	require.NoError(t, err)
	return b
}

func vbHeader(major byte, n int) []byte {
	if n < 24 {
		return []byte{major<<5 | byte(n)}
	}
	return []byte{major<<5 | 24, byte(n)}
}

// vbMultiAsset encodes {policy: {name: quantity}}; names and policies are
// byte strings, so the maps are written by hand. Negative quantities encode
// as CBOR negative integers.
func vbMultiAsset(t *testing.T, policies map[byte]map[string]int64, order []byte) []byte {
	t.Helper()
	out := vbHeader(5, len(policies))
	for _, p := range order {
		out = append(out, vbCbor(t, bytes.Repeat([]byte{p}, 28))...)
		names := policies[p]
		out = append(out, vbHeader(5, len(names))...)
		// One name per policy in these tests keeps the order canonical.
		for name, q := range names {
			out = append(out, vbCbor(t, []byte(name))...)
			out = append(out, vbCbor(t, q)...)
		}
	}
	return out
}

func vbAddr() []byte { return append([]byte{0x61}, bytes.Repeat([]byte{0x11}, 28)...) }

func vbValue(t *testing.T, coin uint64, assets []byte) []byte {
	return append(append([]byte{0x82}, vbCbor(t, coin)...), assets...)
}

func vbOutput(t *testing.T, value []byte) []byte {
	return append(append([]byte{0x82}, vbCbor(t, vbAddr())...), value...)
}

func vbBody(t *testing.T, entries ...[2][]byte) []byte {
	out := vbHeader(5, len(entries))
	for _, e := range entries {
		out = append(out, e[0]...)
		out = append(out, e[1]...)
	}
	return out
}

func vbTx(t *testing.T, body []byte) *conway.ConwayTransaction {
	t.Helper()
	raw := append(append([]byte{0x84}, body...), 0xa0, 0xf5, 0xf6)
	tx, err := conway.NewConwayTransactionFromCbor(raw)
	require.NoError(t, err)
	return tx
}

func vbState(t *testing.T, id common.TransactionInput, output []byte) common.LedgerState {
	t.Helper()
	out, err := babbage.NewBabbageTransactionOutputFromCbor(output)
	require.NoError(t, err)
	return mockledger.NewLedgerStateBuilder().
		WithUtxoById(func(in common.TransactionInput) (common.Utxo, error) {
			if in.Id() == id.Id() && in.Index() == id.Index() {
				return common.Utxo{Id: id, Output: out}, nil
			}
			return common.Utxo{}, common.ErrUtxoNotFound
		}).
		Build()
}

func vbAssets(t *testing.T, got []conway.ValueBalanceAsset) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, a := range got {
		require.True(t, a.Quantity.IsInt64())
		out[string(a.Policy.Bytes()[:1])+string(a.Name)] = a.Quantity.Int64()
	}
	return out
}

// Consumed is the inputs, withdrawals, refunds and positively minted assets.
// Produced is the outputs, fee, deposits, donation and burned assets, as
// getConsumedMaryValue and conwayProducedValue define them.
func TestComputeValueBalance(t *testing.T) {
	t.Parallel()
	txid := bytes.Repeat([]byte{0x01}, 32)
	policyA, policyB := byte(0xaa), byte(0xbb)
	inputAssets := vbMultiAsset(t, map[byte]map[string]int64{policyA: {"a": 5}, policyB: {"b": 4}}, []byte{policyA, policyB})
	outputAssets := vbMultiAsset(t, map[byte]map[string]int64{policyA: {"a": 8}, policyB: {"b": 2}}, []byte{policyA, policyB})
	mint := vbMultiAsset(t, map[byte]map[string]int64{policyA: {"a": 3}, policyB: {"b": -2}}, []byte{policyA, policyB})
	reward := append([]byte{0xe1}, bytes.Repeat([]byte{0x22}, 28)...)
	withdrawals := append(vbHeader(5, 1), append(vbCbor(t, reward), vbCbor(t, uint64(1_000_000))...)...)
	// reg_cert: [7, [0, keyhash], deposit].
	cert := append([]byte{0x83, 0x07, 0x82, 0x00}, append(vbCbor(t, bytes.Repeat([]byte{0x33}, 28)), vbCbor(t, uint64(2_000_000))...)...)
	body := vbBody(t,
		[2][]byte{{0x00}, vbCbor(t, []any{[]any{txid, uint(0)}})},
		[2][]byte{{0x01}, append([]byte{0x81}, vbOutput(t, vbValue(t, 8_300_000, outputAssets))...)},
		[2][]byte{{0x02}, vbCbor(t, uint64(500_000))},
		[2][]byte{{0x04}, append([]byte{0x81}, cert...)},
		[2][]byte{{0x05}, withdrawals},
		[2][]byte{{0x09}, mint},
		[2][]byte{{0x16}, vbCbor(t, uint64(200_000))},
	)
	tx := vbTx(t, body)
	in := tx.Inputs()[0]
	ls := vbState(t, in, vbOutput(t, vbValue(t, 10_000_000, inputAssets)))
	pp := &conway.ConwayProtocolParameters{KeyDeposit: 2_000_000}

	got, err := conway.ComputeValueBalance(tx, 0, ls, pp)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(11_000_000), got.ConsumedCoin)
	require.Equal(t, big.NewInt(11_000_000), got.ProducedCoin)
	require.Equal(t, big.NewInt(2_000_000), got.Deposits)
	require.Equal(t, big.NewInt(0), got.Refunds)
	// Consumed: inputs plus the positive mint. Produced: outputs plus the burn.
	require.Equal(t, map[string]int64{"\xaaa": 8, "\xbbb": 4}, vbAssets(t, got.ConsumedAssets))
	require.Equal(t, map[string]int64{"\xaaa": 8, "\xbbb": 4}, vbAssets(t, got.ProducedAssets))
	require.NoError(t, conway.UtxoValidateValueNotConservedUtxo(tx, 0, ls, pp))
}

func TestComputeValueBalanceRefund(t *testing.T) {
	t.Parallel()
	txid := bytes.Repeat([]byte{0x01}, 32)
	hash := bytes.Repeat([]byte{0x33}, 28)
	// unreg_cert: [8, [0, keyhash], refund].
	cert := append([]byte{0x83, 0x08, 0x82, 0x00}, append(vbCbor(t, hash), vbCbor(t, uint64(2_000_000))...)...)
	body := vbBody(t,
		[2][]byte{{0x00}, vbCbor(t, []any{[]any{txid, uint(0)}})},
		[2][]byte{{0x01}, append([]byte{0x81}, vbOutput(t, vbCbor(t, uint64(6_800_000)))...)},
		[2][]byte{{0x02}, vbCbor(t, uint64(200_000))},
		[2][]byte{{0x04}, append([]byte{0x81}, cert...)},
	)
	tx := vbTx(t, body)
	in := tx.Inputs()[0]
	out, err := babbage.NewBabbageTransactionOutputFromCbor(vbOutput(t, vbCbor(t, uint64(5_000_000))))
	require.NoError(t, err)
	var key common.Blake2b224
	copy(key[:], hash)
	ls := certificateDepositLedgerState{
		LedgerState: mockledger.NewLedgerStateBuilder().
			WithUtxoById(func(common.TransactionInput) (common.Utxo, error) {
				return common.Utxo{Id: in, Output: out}, nil
			}).
			WithStakeCredentialRegistered(key, true).
			Build(),
		deposits: map[certificateDepositCredentialKey]uint64{{
			credType: common.CredentialTypeAddrKeyHash,
			hash:     key,
		}: 2_000_000},
	}
	pp := &conway.ConwayProtocolParameters{KeyDeposit: 2_000_000}
	got, err := conway.ComputeValueBalance(tx, 0, ls, pp)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(2_000_000), got.Refunds)
	require.Equal(t, big.NewInt(7_000_000), got.ConsumedCoin)
	require.Equal(t, big.NewInt(7_000_000), got.ProducedCoin)
	require.Equal(t, big.NewInt(0), got.Deposits)
}
