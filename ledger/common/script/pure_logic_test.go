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
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/gouroboros/ledger/common/script"
	"github.com/blinklabs-io/gouroboros/ledger/conway"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/stretchr/testify/require"
)

func TestSlotToTimeFuncDrivesTxInfoValidityRange(t *testing.T) {
	t.Parallel()
	epoch := time.Unix(1_000, 0)
	var seen []uint64
	converter := script.SlotToTimeFunc(func(slot uint64) (time.Time, error) {
		seen = append(seen, slot)
		return epoch.Add(time.Duration(slot) * time.Second), nil
	})
	tx := &conway.ConwayTransaction{
		Body: conway.ConwayTransactionBody{Ttl: 100},
	}
	info, err := script.NewTxInfoV3FromTransaction(
		converter, tx, nil, 10,
	)
	require.NoError(t, err)
	require.Equal(t, []uint64{100}, seen)
	upperMs := new(big.Int).SetInt64(epoch.Add(100 * time.Second).UnixMilli())
	// Conway upper bound: Finite, exclusive; lower bound NegInf.
	want := data.NewConstr(
		0,
		data.NewConstr(0, data.NewConstr(0), data.NewConstr(1)),
		data.NewConstr(
			0,
			data.NewConstr(1, data.NewInteger(upperMs)),
			data.NewConstr(0),
		),
	)
	require.Equal(t, want, info.ValidRange.ToPlutusData())
	_, err = converter.TimeToSlot(epoch)
	require.Error(t, err)
}

func TestSlotToTimeFuncPropagatesConversionError(t *testing.T) {
	t.Parallel()
	boom := errors.New("slot outside era history")
	tx := &conway.ConwayTransaction{
		Body: conway.ConwayTransactionBody{Ttl: 100},
	}
	_, err := script.NewTxInfoV3FromTransaction(
		script.SlotToTimeFunc(func(uint64) (time.Time, error) {
			return time.Time{}, boom
		}),
		tx, []common.Utxo(nil), 10,
	)
	require.ErrorIs(t, err, boom)
}

func TestTxInfoDataFillsTheScriptContext(t *testing.T) {
	t.Parallel()
	info := script.TxInfoData{Data: data.NewConstr(0, data.NewInteger(big.NewInt(7)))}
	purpose := script.ScriptPurposeMinting{PolicyId: common.NewBlake2b224(make([]byte, 28))}
	redeemer := script.Redeemer{
		Tag:  common.RedeemerTagMint,
		Data: data.NewInteger(big.NewInt(9)),
	}
	t.Run("v1v2", func(t *testing.T) {
		t.Parallel()
		got := script.NewScriptContextV1V2(info, purpose).ToPlutusData()
		want := data.NewConstr(0, info.Data, script.WithWrappedTransactionId{Value: purpose}.ToPlutusData())
		require.Equal(t, want, got)
	})
	t.Run("v3", func(t *testing.T) {
		t.Parallel()
		got := script.NewScriptContextV3(info, redeemer, purpose).ToPlutusData()
		want := data.NewConstr(0, info.Data, redeemer.Data, purpose.ToScriptInfo().ToPlutusData())
		require.Equal(t, want, got)
	})
}
