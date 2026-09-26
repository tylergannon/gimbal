package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

const (
	maxUUIDv7Timestamp = 0xffffffffffff
	maxUUIDv7Sequence  = (uint64(1) << 41) - 1
)

var (
	uuidMu                sync.Mutex
	lastOrdinaryTimestamp int64 = -1
	uuidSequence          uint64
	uuidSequenceSet       bool
)

// UUIDv7 generates a time-ordered UUIDv7 for the current time. Ordinary calls
// never go backwards in time: a clock that regresses keeps the last issued
// timestamp.
func UUIDv7() string {
	timestamp := time.Now().UnixMilli()
	uuidMu.Lock()
	defer uuidMu.Unlock()
	if timestamp < lastOrdinaryTimestamp {
		timestamp = lastOrdinaryTimestamp
	}
	lastOrdinaryTimestamp = timestamp
	return uuidv7Locked(timestamp)
}

// UUIDv7At generates a time-ordered UUIDv7 with an explicit millisecond
// timestamp, which is preserved exactly and does not move the ordinary clock.
func UUIDv7At(timestampMs int64) string {
	uuidMu.Lock()
	defer uuidMu.Unlock()
	return uuidv7Locked(timestampMs)
}

func uuidv7Locked(timestampMs int64) string {
	if timestampMs < 0 || timestampMs > maxUUIDv7Timestamp {
		panic(fmt.Sprintf("model: UUIDv7 timestamp must be between 0 and %d", maxUUIDv7Timestamp))
	}
	if uuidSequenceSet {
		if uuidSequence == maxUUIDv7Sequence {
			panic("model: UUIDv7 generator sequence exhausted")
		}
		uuidSequence++
	} else {
		var seqBytes [5]byte
		if _, err := rand.Read(seqBytes[:]); err != nil {
			panic(err)
		}
		uuidSequence = uint64(seqBytes[0])<<32 |
			uint64(seqBytes[1])<<24 |
			uint64(seqBytes[2])<<16 |
			uint64(seqBytes[3])<<8 |
			uint64(seqBytes[4])
		uuidSequenceSet = true
	}

	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	timestamp := uint64(timestampMs)
	for index := 5; index >= 0; index-- {
		bytes[index] = byte(timestamp >> uint((5-index)*8))
	}
	bytes[6] = 0x70 | byte((uuidSequence>>37)&0x0f)
	bytes[7] = byte((uuidSequence >> 29) & 0xff)
	bytes[8] = 0x80 | byte((uuidSequence>>23)&0x3f)
	bytes[9] = byte((uuidSequence >> 15) & 0xff)
	bytes[10] = byte((uuidSequence >> 7) & 0xff)
	bytes[11] = byte((uuidSequence&0x7f)<<1) | (bytes[11] & 0x01)

	hexed := make([]byte, 32)
	hex.Encode(hexed, bytes[:])
	return string(hexed[0:8]) + "-" + string(hexed[8:12]) + "-" + string(hexed[12:16]) + "-" +
		string(hexed[16:20]) + "-" + string(hexed[20:32])
}

// UUIDv7PrevTimestamp returns the last ordinary timestamp issued, for tests.
func UUIDv7PrevTimestamp() int64 {
	uuidMu.Lock()
	defer uuidMu.Unlock()
	return lastOrdinaryTimestamp
}
