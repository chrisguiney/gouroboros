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

package muxer_test

import (
	"testing"

	"github.com/blinklabs-io/gouroboros/muxer"
)

func TestDecodeSegmentHeader(t *testing.T) {
	t.Parallel()
	hdr := func(ts uint32, proto, length uint16) []byte {
		return []byte{
			byte(ts >> 24), byte(ts >> 16), byte(ts >> 8), byte(ts),
			byte(proto >> 8), byte(proto), byte(length >> 8), byte(length),
		}
	}
	got, err := muxer.DecodeSegmentHeader(hdr(0x01020304, 0x800a, 513))
	if err != nil {
		t.Fatal(err)
	}
	if got.Timestamp != 0x01020304 || got.ProtocolId != 0x800a || got.PayloadLength != 513 ||
		!got.IsResponse() || got.GetProtocolId() != 10 {
		t.Fatalf("header = %+v", got)
	}
	// Bytes past the eighth are the payload, not part of the header.
	if _, err := muxer.DecodeSegmentHeader(append(hdr(1, 2, 3), 0xff)); err != nil {
		t.Fatalf("trailing payload byte: %v", err)
	}
	if _, err := muxer.DecodeSegmentHeader(hdr(1, 2, 3)[:7]); err == nil {
		t.Fatal("seven-byte header decoded")
	}
	if _, err := muxer.DecodeSegmentHeader(hdr(1, 2, 0)); err == nil {
		t.Fatal("zero-length segment decoded")
	}
}
