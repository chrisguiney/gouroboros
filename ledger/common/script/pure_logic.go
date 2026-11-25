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

package script

import (
	"errors"
	"time"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
)

// EvaluatePlutusScriptsPure behaves as EvaluatePlutusScripts but evaluates
// every script with common.EvaluatePlutusScriptPure, so it touches neither the
// shared machine pool nor the shared program cache.
func EvaluatePlutusScriptsPure(
	tx common.Transaction,
	ls common.LedgerState,
	protocolMajor uint,
	costModels map[uint][]int64,
	maxPlutusVersion uint,
	era string,
	budget *common.ExUnits,
) (map[common.RedeemerKey]common.ExUnits, error) {
	return evaluatePlutusScripts(
		tx, ls, protocolMajor, costModels, maxPlutusVersion, era, budget,
		true,
	)
}

// SlotToTimeFunc is a common.SlotState built from a slot-to-time function, for
// callers that convert slots with an era history they hold as a value. The
// TxInfo constructors read only SlotToTime; TimeToSlot reports an error.
type SlotToTimeFunc func(slot uint64) (time.Time, error)

// SlotToTime converts slot with f.
func (f SlotToTimeFunc) SlotToTime(slot uint64) (time.Time, error) {
	return f(slot)
}

// TimeToSlot is not supported by SlotToTimeFunc.
func (SlotToTimeFunc) TimeToSlot(time.Time) (uint64, error) {
	return 0, errors.New("time to slot conversion is not available")
}

// TxInfoData is a TxInfo held as PlutusData, for callers that have the
// TxInfo as serialised data rather than building it from a transaction.
type TxInfoData struct {
	Data data.PlutusData
}

func (TxInfoData) isTxInfo() {}

// ToPlutusData returns the held data.
func (t TxInfoData) ToPlutusData() data.PlutusData { return t.Data }
