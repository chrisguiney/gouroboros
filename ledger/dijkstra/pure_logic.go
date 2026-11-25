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

import "github.com/blinklabs-io/gouroboros/ledger/common"

// UtxoValidationFailures runs every Dijkstra rule and returns every failure,
// not only the first. When families is non-empty only rules of those STS
// families run; see common.UtxoValidationRuleFamily.
func UtxoValidationFailures(
	tx common.Transaction,
	slot uint64,
	ledgerState common.LedgerState,
	protocolParams common.ProtocolParameters,
	families ...common.UtxoValidationFamily,
) []common.UtxoValidationFailure {
	return common.CollectUtxoValidationFailures(
		tx,
		slot,
		ledgerState,
		protocolParams,
		UtxoValidationRules,
		utxoValidationRuleDescriptors,
		families...,
	)
}
