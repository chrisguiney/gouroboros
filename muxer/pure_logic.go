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

package muxer

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// SegmentHeaderLength is the size in bytes of a segment header on the wire.
const SegmentHeaderLength = 8

// ErrZeroLengthSegment is returned for a header announcing an empty payload,
// which the muxer treats as a protocol violation.
var ErrZeroLengthSegment = errors.New("received zero-byte segment payload")

// DecodeSegmentHeader returns the header of a segment from its first
// SegmentHeaderLength bytes. Bytes after those are ignored. It rejects a
// header whose payload length is zero.
func DecodeSegmentHeader(b []byte) (SegmentHeader, error) {
	if len(b) < SegmentHeaderLength {
		return SegmentHeader{}, fmt.Errorf(
			"segment header needs %d bytes, got %d",
			SegmentHeaderLength,
			len(b),
		)
	}
	header := SegmentHeader{
		Timestamp:     binary.BigEndian.Uint32(b[0:4]),
		ProtocolId:    binary.BigEndian.Uint16(b[4:6]),
		PayloadLength: binary.BigEndian.Uint16(b[6:8]),
	}
	if header.PayloadLength == 0 {
		return SegmentHeader{}, ErrZeroLengthSegment
	}
	return header, nil
}
