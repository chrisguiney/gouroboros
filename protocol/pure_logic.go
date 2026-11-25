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

package protocol

import (
	"errors"
	"fmt"
)

// ErrIllegalTransition is wrapped by NextState when the current state is
// unknown or the message type has no matching transition from it.
var ErrIllegalTransition = errors.New("illegal protocol state transition")

// NextState returns the state reached from current on a message of msgType
// and the agency of that next state, using only stateMap.
//
// Transitions are tried in table order and the first one with a matching
// message type wins. A transition's MatchFunc, which may depend on
// stateContext and the message, is consulted only when msg is non-nil; with a
// nil msg the first transition of that type is taken. The agency is
// AgencyNone for a terminal state and for a next state missing from stateMap.
func NextState(
	stateMap StateMap,
	stateContext any,
	current State,
	msgType uint8,
	msg Message,
) (State, ProtocolStateAgency, error) {
	entry, ok := stateMap[current]
	if !ok {
		return State{}, AgencyNone, fmt.Errorf(
			"%w: unknown state %s",
			ErrIllegalTransition,
			current,
		)
	}
	for _, transition := range entry.Transitions {
		if transition.MsgType != msgType {
			continue
		}
		if transition.MatchFunc != nil && msg != nil &&
			!transition.MatchFunc(stateContext, msg) {
			continue
		}
		return transition.NewState,
			stateMap[transition.NewState].Agency,
			nil
	}
	return State{}, AgencyNone, fmt.Errorf(
		"%w: message type %d not allowed in state %s",
		ErrIllegalTransition,
		msgType,
		current,
	)
}
