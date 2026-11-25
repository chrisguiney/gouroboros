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

package protocol_test

import (
	"testing"

	"github.com/blinklabs-io/gouroboros/protocol"
	"github.com/blinklabs-io/gouroboros/protocol/blockfetch"
	"github.com/stretchr/testify/require"
)

func TestNextStateBlockFetch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		from    protocol.State
		msgType uint8
		to      protocol.State
		agency  protocol.ProtocolStateAgency
	}{
		{"request range", blockfetch.StateIdle, blockfetch.MessageTypeRequestRange, blockfetch.StateBusy, protocol.AgencyServer},
		{"client done", blockfetch.StateIdle, blockfetch.MessageTypeClientDone, blockfetch.StateDone, protocol.AgencyNone},
		{"start batch", blockfetch.StateBusy, blockfetch.MessageTypeStartBatch, blockfetch.StateStreaming, protocol.AgencyServer},
		{"no blocks", blockfetch.StateBusy, blockfetch.MessageTypeNoBlocks, blockfetch.StateIdle, protocol.AgencyClient},
		{"block", blockfetch.StateStreaming, blockfetch.MessageTypeBlock, blockfetch.StateStreaming, protocol.AgencyServer},
		{"batch done", blockfetch.StateStreaming, blockfetch.MessageTypeBatchDone, blockfetch.StateIdle, protocol.AgencyClient},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			to, agency, err := protocol.NextState(
				blockfetch.StateMap, nil, tc.from, tc.msgType, nil,
			)
			require.NoError(t, err)
			require.Equal(t, tc.to, to)
			require.Equal(t, tc.agency, agency)
		})
	}
}

func TestNextStateRejectsIllegalTransitions(t *testing.T) {
	t.Parallel()
	_, _, err := protocol.NextState(
		blockfetch.StateMap, nil, blockfetch.StateIdle,
		blockfetch.MessageTypeBlock, nil,
	)
	require.ErrorIs(t, err, protocol.ErrIllegalTransition)
	_, _, err = protocol.NextState(
		blockfetch.StateMap, nil, blockfetch.StateDone,
		blockfetch.MessageTypeRequestRange, nil,
	)
	require.ErrorIs(t, err, protocol.ErrIllegalTransition)
	_, _, err = protocol.NextState(
		blockfetch.StateMap, nil, protocol.NewState(99, "Unknown"),
		blockfetch.MessageTypeRequestRange, nil,
	)
	require.ErrorIs(t, err, protocol.ErrIllegalTransition)
}

func TestNextStateAppliesMatchFunc(t *testing.T) {
	t.Parallel()
	a := protocol.NewState(1, "A")
	b := protocol.NewState(2, "B")
	c := protocol.NewState(3, "C")
	sm := protocol.StateMap{
		a: {
			Agency: protocol.AgencyClient,
			Transitions: []protocol.StateTransition{
				{
					MsgType:  7,
					NewState: b,
					MatchFunc: func(ctx any, _ protocol.Message) bool {
						return ctx == "to-b"
					},
				},
				{MsgType: 7, NewState: c},
			},
		},
		b: {Agency: protocol.AgencyServer},
		c: {Agency: protocol.AgencyNone},
	}
	msg := &protocol.MessageBase{}
	to, _, err := protocol.NextState(sm, "to-b", a, 7, msg)
	require.NoError(t, err)
	require.Equal(t, b, to)
	to, _, err = protocol.NextState(sm, "other", a, 7, msg)
	require.NoError(t, err)
	require.Equal(t, c, to)
}
