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

package common

import (
	"context"
	"fmt"

	"github.com/blinklabs-io/plutigo/cek"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// UtxoValidationFamily names the cardano-ledger STS rule a UTxO validation
// rule belongs to. The mapping follows the Haskell rule that reports the
// corresponding predicate failure, not the Go rule's position in an era list.
type UtxoValidationFamily string

const (
	// UtxoFamilyUtxo is Cardano.Ledger.<Era>.Rules.Utxo.
	UtxoFamilyUtxo UtxoValidationFamily = "UTXO"
	// UtxoFamilyUtxow is Cardano.Ledger.<Era>.Rules.Utxow.
	UtxoFamilyUtxow UtxoValidationFamily = "UTXOW"
	// UtxoFamilyUtxos is Cardano.Ledger.<Era>.Rules.Utxos.
	UtxoFamilyUtxos UtxoValidationFamily = "UTXOS"
	// UtxoFamilyPpup is Cardano.Ledger.Shelley.Rules.Ppup.
	UtxoFamilyPpup UtxoValidationFamily = "PPUP"
	// UtxoFamilyDeleg is the DELEG rule, including deposit checks.
	UtxoFamilyDeleg UtxoValidationFamily = "DELEG"
	// UtxoFamilyPool is Cardano.Ledger.Shelley.Rules.Pool.
	UtxoFamilyPool UtxoValidationFamily = "POOL"
	// UtxoFamilyGovCert is Cardano.Ledger.Conway.Rules.GovCert.
	UtxoFamilyGovCert UtxoValidationFamily = "GOVCERT"
	// UtxoFamilyCerts is the CERTS rule, including withdrawals.
	UtxoFamilyCerts UtxoValidationFamily = "CERTS"
	// UtxoFamilyGov is Cardano.Ledger.Conway.Rules.Gov.
	UtxoFamilyGov UtxoValidationFamily = "GOV"
	// UtxoFamilyLedger is the composed LEDGER rule.
	UtxoFamilyLedger UtxoValidationFamily = "LEDGER"
)

// utxoValidationRuleFamilies maps every rule identifier to its STS family.
var utxoValidationRuleFamilies = map[UtxoValidationRuleId]UtxoValidationFamily{
	UtxoValidationRuleMetadata:                     UtxoFamilyUtxow,
	UtxoValidationRuleRequiredVKeyWitnesses:        UtxoFamilyUtxow,
	UtxoValidationRuleSignatures:                   UtxoFamilyUtxow,
	UtxoValidationRuleProtocolParameterUpdates:     UtxoFamilyPpup,
	UtxoValidationRuleTimeToLive:                   UtxoFamilyUtxo,
	UtxoValidationRuleInputSetEmpty:                UtxoFamilyUtxo,
	UtxoValidationRuleNoDuplicateInputs:            UtxoFamilyUtxo,
	UtxoValidationRuleFeeTooSmall:                  UtxoFamilyUtxo,
	UtxoValidationRuleBadInputs:                    UtxoFamilyUtxo,
	UtxoValidationRuleNativeScripts:                UtxoFamilyUtxow,
	UtxoValidationRuleScriptWitnesses:              UtxoFamilyUtxow,
	UtxoValidationRuleWrongNetwork:                 UtxoFamilyUtxo,
	UtxoValidationRuleWrongNetworkWithdrawal:       UtxoFamilyUtxo,
	UtxoValidationRuleValueNotConserved:            UtxoFamilyUtxo,
	UtxoValidationRuleValueNotConservedLegacyMode:  UtxoFamilyUtxo,
	UtxoValidationRuleTreasuryDonationScripts:      UtxoFamilyUtxo,
	UtxoValidationRuleOutputTooSmall:               UtxoFamilyUtxo,
	UtxoValidationRuleOutputBootAddrAttrsTooBig:    UtxoFamilyUtxo,
	UtxoValidationRuleMaxTxSize:                    UtxoFamilyUtxo,
	UtxoValidationRuleDelegation:                   UtxoFamilyDeleg,
	UtxoValidationRuleMIRGenesisQuorum:             UtxoFamilyUtxow,
	UtxoValidationRuleWithdrawals:                  UtxoFamilyCerts,
	UtxoValidationRuleBatchWithdrawals:             UtxoFamilyCerts,
	UtxoValidationRuleAccountBalanceIntervals:      UtxoFamilyCerts,
	UtxoValidationRuleCertificateDeposits:          UtxoFamilyDeleg,
	UtxoValidationRuleOutsideValidityInterval:      UtxoFamilyUtxo,
	UtxoValidationRuleOutsideForecast:              UtxoFamilyUtxo,
	UtxoValidationRuleOutputTooBig:                 UtxoFamilyUtxo,
	UtxoValidationRuleIsValidFlag:                  UtxoFamilyUtxos,
	UtxoValidationRuleCollateralVKeyWitnesses:      UtxoFamilyUtxow,
	UtxoValidationRuleCollateralKeyLocked:          UtxoFamilyUtxo,
	UtxoValidationRuleRedeemerAndScriptWitnesses:   UtxoFamilyUtxow,
	UtxoValidationRuleCostModelsPresent:            UtxoFamilyUtxos,
	UtxoValidationRuleScriptDataHash:               UtxoFamilyUtxow,
	UtxoValidationRuleInsufficientCollateral:       UtxoFamilyUtxo,
	UtxoValidationRuleCollateralContainsNonAda:     UtxoFamilyUtxo,
	UtxoValidationRuleNoCollateralInputs:           UtxoFamilyUtxo,
	UtxoValidationRuleExUnitsTooBig:                UtxoFamilyUtxo,
	UtxoValidationRuleExtraneousRedeemers:          UtxoFamilyUtxow,
	UtxoValidationRulePlutusScripts:                UtxoFamilyUtxos,
	UtxoValidationRuleInlineDatumsWithPlutusV1:     UtxoFamilyUtxos,
	UtxoValidationRuleDisjointRefInputs:            UtxoFamilyUtxo,
	UtxoValidationRuleCollateralEqBalance:          UtxoFamilyUtxo,
	UtxoValidationRulePtrPresentInCollateralReturn: UtxoFamilyUtxo,
	UtxoValidationRuleTooManyCollateralInputs:      UtxoFamilyUtxo,
	UtxoValidationRuleMalformedReferenceScripts:    UtxoFamilyUtxow,
	UtxoValidationRuleCurrentTreasuryValue:         UtxoFamilyLedger,
	UtxoValidationRuleProposalProcedures:           UtxoFamilyGov,
	UtxoValidationRuleGovActionWellFormedness:      UtxoFamilyGov,
	UtxoValidationRuleHardForkCanFollow:            UtxoFamilyGov,
	UtxoValidationRuleProposalAncestry:             UtxoFamilyGov,
	UtxoValidationRuleProposalDeposit:              UtxoFamilyGov,
	UtxoValidationRuleProposalNetworkIds:           UtxoFamilyGov,
	UtxoValidationRuleProposalReturnAccounts:       UtxoFamilyGov,
	UtxoValidationRuleProposalReturnAddressShape:   UtxoFamilyGov,
	UtxoValidationRuleEmptyTreasuryWithdrawals:     UtxoFamilyGov,
	UtxoValidationRuleBootstrapAllowedGovActions:   UtxoFamilyGov,
	UtxoValidationRuleConwayFeaturesWithPlutusV1V2: UtxoFamilyUtxos,
	UtxoValidationRuleTransactionNetworkId:         UtxoFamilyUtxo,
	UtxoValidationRuleSupplementalDatums:           UtxoFamilyUtxow,
	UtxoValidationRuleCommitteeCertificates:        UtxoFamilyGovCert,
	UtxoValidationRuleUnknownVoters:                UtxoFamilyGov,
	UtxoValidationRuleUnknownGovActionIds:          UtxoFamilyGov,
	UtxoValidationRuleVotingOnExpiredGovAction:     UtxoFamilyGov,
	UtxoValidationRuleBootstrapVotingRestrictions:  UtxoFamilyGov,
	UtxoValidationRuleStakePoolVotingRestrictions:  UtxoFamilyGov,
	UtxoValidationRuleCCVotingRestrictions:         UtxoFamilyGov,
	UtxoValidationRuleUnelectedCommitteeVoters:     UtxoFamilyGov,
	UtxoValidationRuleRefScriptSizePerTx:           UtxoFamilyLedger,
	UtxoValidationRuleRequiredRedeemers:            UtxoFamilyUtxow,
	UtxoValidationRulePoolCertificates:             UtxoFamilyPool,
}

// UtxoValidationRuleFamily returns the STS family of a rule identifier and
// whether the identifier is known.
func UtxoValidationRuleFamily(
	id UtxoValidationRuleId,
) (UtxoValidationFamily, bool) {
	family, ok := utxoValidationRuleFamilies[id]
	return family, ok
}

// UtxoValidationFailure is one failed rule reported by
// CollectUtxoValidationFailures. RuleIndex is the rule's position in the
// supplied rule list, or -1 when the failure concerns the lists themselves.
type UtxoValidationFailure struct {
	RuleIndex int
	RuleId    UtxoValidationRuleId
	Family    UtxoValidationFamily
	Err       error
}

// runUtxoValidationRules calls each rule in order with a transaction-scoped
// ledger state, skipping nil entries. It stops when onFailure returns false.
func runUtxoValidationRules(
	tx Transaction,
	slot uint64,
	ledgerState LedgerState,
	protocolParams ProtocolParameters,
	rules []UtxoValidationRuleFunc,
	signatures *PreverifiedSignatures,
	onFailure func(index int, err error) (cont bool),
) {
	ledgerState = newTransactionScopedLedgerState(ledgerState, signatures)
	for i, rule := range rules {
		if rule == nil {
			continue
		}
		if err := rule(tx, slot, ledgerState, protocolParams); err != nil {
			if !onFailure(i, err) {
				return
			}
		}
	}
}

// CollectUtxoValidationFailures runs every rule and returns every failure
// instead of stopping at the first. descriptors must be the rule identifiers
// for rules in the same order, as each era's UtxoValidationRuleDescriptors
// and UtxoValidationRules are. When families is non-empty only rules of those
// families run. Mismatched lengths yield a single failure with RuleIndex -1.
func CollectUtxoValidationFailures(
	tx Transaction,
	slot uint64,
	ledgerState LedgerState,
	protocolParams ProtocolParameters,
	rules []UtxoValidationRuleFunc,
	descriptors []UtxoValidationRuleDescriptor,
	families ...UtxoValidationFamily,
) []UtxoValidationFailure {
	if len(rules) != len(descriptors) {
		return []UtxoValidationFailure{{
			RuleIndex: -1,
			Err: fmt.Errorf(
				"%d rules but %d descriptors",
				len(rules),
				len(descriptors),
			),
		}}
	}
	selected := rules
	if len(families) > 0 {
		selected = make([]UtxoValidationRuleFunc, len(rules))
		for i, d := range descriptors {
			family, _ := UtxoValidationRuleFamily(d.Id)
			for _, want := range families {
				if want == family {
					selected[i] = rules[i]
					break
				}
			}
		}
	}
	var failures []UtxoValidationFailure
	runUtxoValidationRules(
		tx, slot, ledgerState, protocolParams, selected, nil,
		func(i int, err error) bool {
			family, _ := UtxoValidationRuleFamily(descriptors[i].Id)
			failures = append(failures, UtxoValidationFailure{
				RuleIndex: i,
				RuleId:    descriptors[i].Id,
				Family:    family,
				Err:       err,
			})
			return true
		},
	)
	return failures
}

// plutusRuntime selects how evaluatePlutusScript prepares and runs a program.
// The pooled runtime shares prepared programs and machines process-wide; the
// pure runtime shares nothing between calls.
type plutusRuntime struct {
	prepare func(
		innerScript []byte,
		language lang.LanguageVersion,
		evalContext *cek.EvalContext,
	) (*cek.PreparedProgram, error)
	run func(
		ctx context.Context,
		language lang.LanguageVersion,
		evalContext *cek.EvalContext,
		budget cek.ExBudget,
		program *cek.PreparedProgram,
		trustedDataArguments ...data.PlutusData,
	) (cek.ExBudget, bool, error)
}

var pooledPlutusRuntime = plutusRuntime{
	prepare: decodePreparedPlutusProgram,
	run:     runPooledPreparedContext,
}

var purePlutusRuntime = plutusRuntime{
	prepare: preparePlutusProgramUncached,
	run:     runFreshPreparedContext,
}

// runFreshPreparedContext runs program on a newly built Machine.
func runFreshPreparedContext(
	ctx context.Context,
	language lang.LanguageVersion,
	evalContext *cek.EvalContext,
	budget cek.ExBudget,
	program *cek.PreparedProgram,
	trustedDataArguments ...data.PlutusData,
) (cek.ExBudget, bool, error) {
	machine := cek.NewMachine[syn.DeBruijn](language, 200, evalContext)
	machine.SetExBudget(budget)
	isUnit, runErr := cek.RunPreparedScriptContext(
		ctx,
		machine,
		program,
		trustedDataArguments...,
	)
	return budget.Sub(&machine.ExBudget), isUnit, runErr
}

// evaluatePlutusScript is the single evaluation path for every Plutus
// language version. PlutusV1 and PlutusV2 take datum, redeemer and context
// arguments and ignore the result value; V3 and later take only the context,
// reject trailing bytes after the script wrapper, and require a unit result.
// A nil datum is omitted from the argument list. Data arguments must be
// finite, valid Plutus data sourced from the ledger.
func evaluatePlutusScript(
	ctx context.Context,
	runtime plutusRuntime,
	language lang.LanguageVersion,
	script []byte,
	datum data.PlutusData,
	redeemer data.PlutusData,
	scriptContext data.PlutusData,
	budget ExUnits,
	evalContext *cek.EvalContext,
) (ExUnits, error) {
	if err := ctx.Err(); err != nil {
		return ExUnits{}, err
	}
	legacy := language == lang.LanguageVersionV1 ||
		language == lang.LanguageVersionV2
	var used ExUnits
	// Decode raw script as bytestring to get actual script bytes
	innerScript, err := decodePlutusScript(script, !legacy)
	if err != nil {
		return used, fmt.Errorf("decode cbor: %w", err)
	}
	program, err := runtime.prepare(innerScript, language, evalContext)
	if err != nil {
		return used, fmt.Errorf("decode script: %w", err)
	}
	var arguments []data.PlutusData
	if legacy {
		// Decode preserves each container's definite/indefinite length
		// choice so a decoded value re-encodes to its original bytes, but
		// cardano-ledger rebuilds these values, which equals the package
		// default encoding. Un-normalized wire data can serialise to
		// different bytes than the reference for the same value, and a
		// script that hashes SerialiseData output would diverge. V3
		// carries its redeemer inside the already-normalized context.
		datum = data.NormalizeShared(datum)
		redeemer = data.NormalizeShared(redeemer)
		arguments = make([]data.PlutusData, 0, 3)
		if datum != nil {
			arguments = append(arguments, datum)
		}
		arguments = append(arguments, redeemer, scriptContext)
	} else {
		arguments = []data.PlutusData{scriptContext}
	}
	consumed, isUnit, runErr := runtime.run(
		ctx,
		language,
		evalContext,
		exBudgetFromUnits(budget),
		program,
		arguments...,
	)
	used.Memory = consumed.Mem
	used.Steps = consumed.Cpu
	if runErr == nil && !legacy && !isUnit {
		runErr = PlutusScriptResultNotUnitError{}
	}
	if runErr != nil {
		return used, fmt.Errorf("execute script: %w", runErr)
	}
	return used, nil
}

// PlutusScriptEvalInput is the complete input of one script evaluation.
type PlutusScriptEvalInput struct {
	// Language is the Plutus language version, such as lang.LanguageVersionV3.
	Language lang.LanguageVersion
	// Script is the script as stored on chain: a CBOR byte string wrapping
	// the flat-encoded program.
	Script []byte
	// Datum is the spending datum for PlutusV1 and PlutusV2; nil omits it.
	Datum data.PlutusData
	// Redeemer is the redeemer argument for PlutusV1 and PlutusV2; PlutusV3
	// and later carry it inside ScriptContext.
	Redeemer data.PlutusData
	// ScriptContext is the final script argument.
	ScriptContext data.PlutusData
	// Budget is the execution budget, enforced exactly including zero.
	Budget ExUnits
}

// EvaluatePlutusScriptPure evaluates one script using only its arguments. It
// builds a fresh evaluation context and machine and uses neither the shared
// machine pool nor the shared program cache, so the result depends only on
// the input, protocolMajor and costModelParams. The returned ExUnits are the
// units consumed, reported even when evaluation fails.
func EvaluatePlutusScriptPure(
	ctx context.Context,
	in PlutusScriptEvalInput,
	protocolMajor uint,
	costModelParams []int64,
) (ExUnits, error) {
	evalContext, err := cek.NewEvalContext(
		in.Language,
		cek.ProtoVersion{Major: protocolMajor},
		costModelParams,
	)
	if err != nil {
		return ExUnits{}, fmt.Errorf("build evaluation context: %w", err)
	}
	return evaluatePlutusScript(
		ctx,
		purePlutusRuntime,
		in.Language,
		in.Script,
		in.Datum,
		in.Redeemer,
		in.ScriptContext,
		in.Budget,
		evalContext,
	)
}

// ResolvedUtxos is a UtxoState backed by an explicit list of resolved UTxOs,
// for callers that hold the inputs of a transaction as values rather than a
// ledger database.
type ResolvedUtxos []Utxo

// UtxoById returns the UTxO with the same transaction ID and index as input,
// or an error wrapping ErrUtxoNotFound.
func (r ResolvedUtxos) UtxoById(input TransactionInput) (Utxo, error) {
	for _, utxo := range r {
		if utxo.Id != nil &&
			utxo.Id.Id() == input.Id() &&
			utxo.Id.Index() == input.Index() {
			return utxo, nil
		}
	}
	return Utxo{}, fmt.Errorf("%w: %s", ErrUtxoNotFound, input)
}
