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
	"slices"
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

// CollectedScript is a script a redeemer runs, with the arguments it is
// applied to, as found by CollectPlutusScripts.
type CollectedScript struct {
	// Key is the redeemer that runs the script.
	Key common.RedeemerKey
	// Purpose is what the redeemer points at.
	Purpose ScriptPurpose
	// ScriptHash is the hash of Script.
	ScriptHash common.ScriptHash
	Script     common.Script
	// Datum is the spending datum of a V1 or V2 script. It is nil for any
	// other script.
	Datum data.PlutusData
	// Redeemer is the redeemer data a V1 or V2 script is applied to. It is
	// nil for a V3 script, whose context holds the redeemer.
	Redeemer data.PlutusData
	// Context is the script context. It is the last argument of every script.
	Context data.PlutusData
	// ExUnits are the units the redeemer declares.
	ExUnits common.ExUnits
}

// Language is the Plutus language number of the script: 1, 2 or 3.
func (s CollectedScript) Language() uint {
	switch s.Script.(type) {
	case common.PlutusV3Script:
		return 3
	case common.PlutusV2Script:
		return 2
	}
	return 1
}

// CollectResult is the outcome of collecting one redeemer's script. Script is
// nil, with a nil Err, when the redeemer's script is not available.
type CollectResult struct {
	Key    common.RedeemerKey
	Script *CollectedScript
	Err    error
}

// CollectPlutusScripts collects the script of every redeemer of tx on its
// own, in redeemer order, and reports the outcome of each; a redeemer that
// cannot be collected does not stop the others. The error is for a failure
// before any redeemer is looked at, such as an input that does not resolve.
// A redeemer whose script is not available has a nil Script and a nil Err, as
// EvaluatePlutusScripts skips it. maxPlutusVersion is the highest language
// number the era supports.
func CollectPlutusScripts(
	tx common.Transaction,
	ls common.LedgerState,
	protocolMajor uint,
	maxPlutusVersion uint,
	era string,
) ([]CollectResult, error) {
	collector, err := newPlutusCollector(
		tx, ls, protocolMajor, maxPlutusVersion, era,
	)
	if err != nil || collector == nil {
		return nil, err
	}
	var out []CollectResult
	for key, value := range collector.redeemers.Iter() {
		collected, err := collector.collect(key, value)
		out = append(out, CollectResult{Key: key, Script: collected, Err: err})
	}
	slices.SortStableFunc(out, func(a, b CollectResult) int {
		if a.Key.Tag != b.Key.Tag {
			return int(a.Key.Tag) - int(b.Key.Tag)
		}
		return int(a.Key.Index) - int(b.Key.Index)
	})
	return out, nil
}

// EvaluateCollectedScriptPure runs one collected script as
// EvaluatePlutusScriptsPure does, independently of every other script of the
// transaction. costModels is indexed by language as in
// EvaluatePlutusScripts, and budget, when set, replaces the units the
// redeemer declares. A failed run is a common.PlutusScriptFailedError.
func EvaluateCollectedScriptPure(
	s CollectedScript,
	protocolMajor uint,
	costModels map[uint][]int64,
	budget *common.ExUnits,
) (common.ExUnits, error) {
	return s.execute(true, protocolMajor, costModels, budget)
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
