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
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/blinklabs-io/gouroboros/ledger/common"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

// EvaluatePlutusScripts executes every redeemer against its required script.
// The transaction's IsValid declaration is intentionally ignored.
// maxPlutusVersion is the highest language number supported by the era.
func EvaluatePlutusScripts(
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
		false,
	)
}

// evaluatePlutusScripts is the shared body of EvaluatePlutusScripts and
// EvaluatePlutusScriptsPure. pure selects whether each script runs through
// the process-wide machine pool and program cache or through
// common.EvaluatePlutusScriptPure; everything before the script run is shared.
func evaluatePlutusScripts(
	tx common.Transaction,
	ls common.LedgerState,
	protocolMajor uint,
	costModels map[uint][]int64,
	maxPlutusVersion uint,
	era string,
	budget *common.ExUnits,
	pure bool,
) (map[common.RedeemerKey]common.ExUnits, error) {
	used := make(map[common.RedeemerKey]common.ExUnits)
	collector, err := newPlutusCollector(
		tx, ls, protocolMajor, maxPlutusVersion, era,
	)
	if err != nil {
		return nil, err
	}
	if collector == nil {
		return used, nil
	}
	// Each redeemer is collected and then run before the next is collected,
	// so the first failure, whether in collection or execution, is the one
	// reported.
	for redeemerKey, redeemerValue := range collector.redeemers.Iter() {
		collected, err := collector.collect(redeemerKey, redeemerValue)
		if err != nil {
			return nil, err
		}
		if collected == nil {
			continue
		}
		usedUnits, err := collected.execute(
			pure, protocolMajor, costModels, budget,
		)
		if err != nil {
			return nil, err
		}
		used[redeemerKey] = usedUnits
	}
	return used, nil
}

// plutusCollector holds what collecting a script needs of one transaction:
// the resolved inputs, the scripts available to it and the TxInfo of each
// language, built the first time a script of that language is collected.
type plutusCollector struct {
	tx               common.Transaction
	ls               common.LedgerState
	protocolMajor    uint
	maxPlutusVersion uint
	era              string
	redeemers        common.TransactionWitnessRedeemers

	resolvedInputs     []common.Utxo
	resolvedInputsMap  map[string]common.Utxo
	availableScripts   map[common.ScriptHash]common.Script
	sortedInputs       []common.TransactionInput
	assetMint          common.MultiAsset[common.MultiAssetTypeMint]
	withdrawals        map[*common.Address]*big.Int
	votes              common.VotingProcedures
	proposalProcedures []common.ProposalProcedure
	certificates       []common.Certificate
	witnessDatums      map[common.Blake2b256]*common.Datum

	// The TxInfo is cached so each redeemer reuses one PlutusData
	// conversion of it.
	txInfoV1, txInfoV2, txInfoV3 *CachedTxInfo
}

// newPlutusCollector prepares to collect the scripts of tx's redeemers. It
// returns a nil collector when there is nothing to collect: no witnesses or
// no redeemers.
func newPlutusCollector(
	tx common.Transaction,
	ls common.LedgerState,
	protocolMajor uint,
	maxPlutusVersion uint,
	era string,
) (*plutusCollector, error) {
	if tx == nil {
		return nil, errors.New("nil transaction")
	}
	witnesses := tx.Witnesses()
	if witnesses == nil {
		return nil, nil
	}
	redeemers := witnesses.Redeemers()
	if redeemers == nil {
		return nil, nil
	}

	// Count redeemers to see if we have any scripts to execute
	redeemerCount := 0
	for range redeemers.Iter() {
		redeemerCount++
	}
	if redeemerCount == 0 {
		return nil, nil
	}

	// Resolve all inputs (regular + reference) for building script context
	inputsResolved, refInputsResolved, err := ResolveTxInputs(tx, ls)
	if err != nil {
		return nil, err
	}
	resolvedInputs := ConcatResolvedInputs(
		inputsResolved,
		refInputsResolved,
	)
	resolvedInputsMap := make(
		map[string]common.Utxo,
		len(inputsResolved)+len(refInputsResolved),
	)
	for i, input := range tx.Inputs() {
		resolvedInputsMap[input.String()] = inputsResolved[i]
	}
	for i, refInput := range tx.ReferenceInputs() {
		resolvedInputsMap[refInput.String()] = refInputsResolved[i]
	}

	assetMint := tx.AssetMint()
	if assetMint == nil {
		assetMint = &common.MultiAsset[common.MultiAssetTypeMint]{}
	}

	// Build witness datums map for datum lookup
	plutusData := witnesses.PlutusData()
	witnessDatums := make(map[common.Blake2b256]*common.Datum)
	for i := range plutusData {
		datum := plutusData[i]
		witnessDatums[datum.Hash()] = &datum
	}

	return &plutusCollector{
		tx:                tx,
		ls:                ls,
		protocolMajor:     protocolMajor,
		maxPlutusVersion:  maxPlutusVersion,
		era:               era,
		redeemers:         redeemers,
		resolvedInputs:    resolvedInputs,
		resolvedInputsMap: resolvedInputsMap,
		// Collect all available scripts (witness scripts + reference scripts)
		availableScripts: AvailablePlutusScripts(tx, resolvedInputs),
		// The Cardano ledger spec requires spend redeemer indices to
		// reference positions in the canonically sorted input list.
		sortedInputs:       SortInputs(tx.Inputs()),
		assetMint:          *assetMint,
		withdrawals:        tx.Withdrawals(),
		votes:              tx.VotingProcedures(),
		proposalProcedures: tx.ProposalProcedures(),
		certificates:       tx.Certificates(),
		witnessDatums:      witnessDatums,
	}, nil
}

// collect resolves one redeemer to the script it runs and the arguments the
// script is applied to. It returns a nil script and no error for a redeemer
// whose script is not available, which the missing-script-witness rule
// reports.
func (c *plutusCollector) collect(
	redeemerKey common.RedeemerKey,
	redeemerValue common.RedeemerValue,
) (*CollectedScript, error) {
	// Build script purpose for this redeemer
	purpose, err := BuildScriptPurpose(
		redeemerKey,
		c.resolvedInputsMap,
		c.sortedInputs,
		c.assetMint,
		c.certificates,
		c.withdrawals,
		c.votes,
		c.proposalProcedures,
		c.witnessDatums,
		c.protocolMajor,
	)
	if err != nil {
		// Redeemer doesn't match any valid purpose (index out of bounds, etc.)
		return nil, common.ExtraRedeemerError{RedeemerKey: redeemerKey}
	}

	// Check if the purpose actually requires a script
	// If not, this redeemer is "extra" (mismatched)
	switch p := purpose.(type) {
	case ScriptPurposeSpending:
		// For spending purposes, verify the input is at a script address
		if p.Input.Output != nil {
			addr := p.Input.Output.Address()
			if (addr.Type() & common.AddressTypeScriptBit) == 0 {
				// Input is at a key address, not a script address
				return nil, common.ExtraRedeemerError{RedeemerKey: redeemerKey}
			}
		}
	case ScriptPurposeCertifying:
		// For certifying purposes, check if the certificate has a script
		// credential. ScriptHash() returns an empty hash for key credentials.
		if p.ScriptHash() == (common.ScriptHash{}) {
			return nil, common.ExtraRedeemerError{RedeemerKey: redeemerKey}
		}
	case ScriptPurposeRewarding:
		// For rewarding purposes, check if the credential is a script
		if p.StakeCredential.CredType != common.CredentialTypeScriptHash {
			return nil, common.ExtraRedeemerError{RedeemerKey: redeemerKey}
		}
	case ScriptPurposeProposing:
		// For proposing purposes, check if the proposal has a policy script
		// If not (empty ScriptHash), this redeemer is "extra"
		if p.ScriptHash() == (common.ScriptHash{}) {
			return nil, common.ExtraRedeemerError{RedeemerKey: redeemerKey}
		}
	case ScriptPurposeVoting:
		if !VoterUsesScriptCredential(p.Voter) {
			return nil, common.ExtraRedeemerError{RedeemerKey: redeemerKey}
		}
	}

	// Find the script for this purpose
	scriptHash := purpose.ScriptHash()
	plutusScript, ok := c.availableScripts[scriptHash]
	if !ok {
		// Missing script should be caught by MissingScriptWitnesses
		return nil, nil
	}

	// Get datum for V1/V2 scripts (spending purpose only)
	var datum data.PlutusData
	var spendInput common.TransactionInput
	if spendPurpose, ok := purpose.(ScriptPurposeSpending); ok {
		if spendPurpose.Datum != nil {
			datum = spendPurpose.Datum
		}
		spendInput = spendPurpose.Input.Id
	}

	collected := &CollectedScript{
		Key:        redeemerKey,
		Purpose:    purpose,
		ScriptHash: scriptHash,
		Script:     plutusScript,
		ExUnits:    redeemerValue.ExUnits,
	}
	switch plutusScript.(type) {
	case common.PlutusV4Script:
		return nil, common.PlutusScriptValidationUnsupportedError{Era: c.era}
	case common.PlutusV3Script:
		if c.maxPlutusVersion < 3 {
			return nil, common.PlutusScriptValidationUnsupportedError{
				Era: c.era,
			}
		}
		// Build V3 TxInfo lazily
		if c.txInfoV3 == nil {
			if err := ValidatePlutusV3ReferenceInputs(
				c.tx,
				c.protocolMajor,
			); err != nil {
				return nil, common.ScriptContextConstructionError{Err: err}
			}
			txInfo, err := NewTxInfoV3FromTransaction(
				c.ls,
				c.tx,
				c.resolvedInputs,
				c.protocolMajor,
			)
			if err != nil {
				return nil, common.ScriptContextConstructionError{Err: err}
			}
			c.txInfoV3 = NewCachedTxInfo(txInfo)
		}
		// Build V3 context
		redeemer := Redeemer{
			Tag:     redeemerKey.Tag,
			Index:   redeemerKey.Index,
			Data:    redeemerValue.Data.Data,
			ExUnits: redeemerValue.ExUnits,
		}
		collected.Context = NewScriptContextV3(
			c.txInfoV3, redeemer, purpose,
		).ToPlutusData()
	case common.PlutusV2Script:
		if c.maxPlutusVersion < 2 {
			return nil, common.PlutusScriptValidationUnsupportedError{
				Era: c.era,
			}
		}
		// V2 scripts require a datum for spending purposes
		if _, isSpend := purpose.(ScriptPurposeSpending); isSpend &&
			datum == nil {
			return nil, common.MissingDatumForSpendingScriptError{
				ScriptHash: scriptHash,
				Input:      spendInput,
			}
		}
		// Build V2 TxInfo lazily
		if c.txInfoV2 == nil {
			txInfo, err := NewTxInfoV2FromTransaction(
				c.ls, c.tx, c.resolvedInputs,
				StrictValidityUpperBoundForTransaction(c.tx),
				c.protocolMajor,
			)
			if err != nil {
				return nil, common.ScriptContextConstructionError{Err: err}
			}
			c.txInfoV2 = NewCachedTxInfo(txInfo)
		}
		// Build V1V2 context
		collected.Datum = datum
		collected.Redeemer = redeemerValue.Data.Data
		collected.Context = NewScriptContextV1V2(
			c.txInfoV2, purpose,
		).ToPlutusData()
	case common.PlutusV1Script:
		if c.maxPlutusVersion < 1 {
			return nil, common.PlutusScriptValidationUnsupportedError{
				Era: c.era,
			}
		}
		// V1 scripts require a datum for spending purposes
		if _, isSpend := purpose.(ScriptPurposeSpending); isSpend &&
			datum == nil {
			return nil, common.MissingDatumForSpendingScriptError{
				ScriptHash: scriptHash,
				Input:      spendInput,
			}
		}
		// Build V1 TxInfo lazily
		if c.txInfoV1 == nil {
			txInfo, err := NewTxInfoV1FromTransaction(
				c.ls, c.tx, c.resolvedInputs,
				StrictValidityUpperBoundForTransaction(c.tx),
				c.protocolMajor,
			)
			if err != nil {
				return nil, common.ScriptContextConstructionError{Err: err}
			}
			c.txInfoV1 = NewCachedTxInfo(txInfo)
		}
		// Build V1V2 context
		collected.Datum = datum
		collected.Redeemer = redeemerValue.Data.Data
		collected.Context = NewScriptContextV1V2(
			c.txInfoV1, purpose,
		).ToPlutusData()
	default:
		return nil, nil
	}
	return collected, nil
}

// execute runs the collected script. The budget, when set, replaces the
// units the redeemer declares. A failed run is a PlutusScriptFailedError
// naming the redeemer.
func (s CollectedScript) execute(
	pure bool,
	protocolMajor uint,
	costModels map[uint][]int64,
	budget *common.ExUnits,
) (common.ExUnits, error) {
	units := s.ExUnits
	if budget != nil {
		units = *budget
	}
	var language lang.LanguageVersion
	var costModelIndex uint
	switch s.Script.(type) {
	case common.PlutusV3Script:
		language, costModelIndex = lang.LanguageVersionV3, 2
	case common.PlutusV2Script:
		language, costModelIndex = lang.LanguageVersionV2, 1
	default:
		language, costModelIndex = lang.LanguageVersionV1, 0
	}
	usedUnits, err := runPlutusScript(
		pure, language, protocolMajor, costModels[costModelIndex],
		s.Script, s.Datum, s.Redeemer, s.Context, units,
	)
	if err != nil {
		return common.ExUnits{}, common.PlutusScriptFailedError{
			ScriptHash: s.ScriptHash,
			Tag:        s.Key.Tag,
			Index:      s.Key.Index,
			Err:        err,
		}
	}
	return usedUnits, nil
}

// runPlutusScript evaluates one script. When pure is false it uses a pooled
// evaluation context and the script's pooled Evaluate method; otherwise it
// builds everything fresh. The script's own Evaluate methods and
// common.EvaluatePlutusScriptPure share one evaluation routine, so the two
// differ only in caching.
func runPlutusScript(
	pure bool,
	language lang.LanguageVersion,
	protocolMajor uint,
	costModelParams []int64,
	script common.Script,
	datum data.PlutusData,
	redeemer data.PlutusData,
	scriptContext data.PlutusData,
	units common.ExUnits,
) (common.ExUnits, error) {
	if pure {
		return common.EvaluatePlutusScriptPure(
			context.Background(),
			common.PlutusScriptEvalInput{
				Language:      language,
				Script:        script.RawScriptBytes(),
				Datum:         datum,
				Redeemer:      redeemer,
				ScriptContext: scriptContext,
				Budget:        units,
			},
			protocolMajor,
			costModelParams,
		)
	}
	evalContext, err := common.PooledEvalContext(
		language,
		protocolMajor,
		costModelParams,
	)
	if err != nil {
		return common.ExUnits{}, fmt.Errorf(
			"build evaluation context: %w",
			err,
		)
	}
	switch s := script.(type) {
	case common.PlutusV1Script:
		return s.Evaluate(
			datum, data.NormalizeShared(redeemer), scriptContext, units,
			evalContext,
		)
	case common.PlutusV2Script:
		return s.Evaluate(
			datum, data.NormalizeShared(redeemer), scriptContext, units,
			evalContext,
		)
	case common.PlutusV3Script:
		return s.Evaluate(scriptContext, units, evalContext)
	default:
		return common.ExUnits{}, fmt.Errorf(
			"unsupported script type %T",
			script,
		)
	}
}
