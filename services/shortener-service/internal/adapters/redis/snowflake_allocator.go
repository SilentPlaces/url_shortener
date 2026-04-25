package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/arminaray/url_shortener/services/shortener-service/internal/domain/ports"
)

const (
	// Start Epoch : 2021-01-01 00:00:00 UTC in milliseconds
	snowflakeEpoch = 1609459200000

	// Bit allocations for Snow Flake 64-bit ID:
	// - Timestamp: 41 bits (milliseconds since epoch)
	// - Machine ID: 10 bits (0-1023 machines)
	// - Sequence: 12 bits (0-4095 per millisecond)
	machineIDBits  = 10
	sequenceBits   = 12
	machineIDShift = sequenceBits
	timestampShift = sequenceBits + machineIDBits

	// Maximum values (calculated using bitwise operations)
	// maxMachineID = 2^10 - 1 = 1023
	// maxSequence = 2^12 - 1 = 4095
	// Formula: -1 ^ (-1 << bits) creates a mask of all 1s for the given number of bits
	maxMachineID = -1 ^ (-1 << machineIDBits) // 1023
	maxSequence  = -1 ^ (-1 << sequenceBits)  // 4095
)

// SnowflakeIDAllocator generates distributed unique IDs using Twitter Snowflake algorithm
// Format: 64-bit ID = [Timestamp (41 bits)][Machine ID (10 bits)][Sequence (12 bits)]
type SnowflakeIDAllocator struct {
	machineID int64
	sequence  int64
	lastTime  int64
	mutex     sync.Mutex
}

// NewSnowflakeIDAllocator creates a new Snowflake ID allocator
// machineID must be between 0 and 1023 (unique per server/process)
func NewSnowflakeIDAllocator(machineID int64) (ports.IDAllocator, error) {
	if machineID < 0 || machineID > maxMachineID {
		return nil, fmt.Errorf("machineID must be between 0 and %d, got %d", maxMachineID, machineID)
	}

	return &SnowflakeIDAllocator{
		machineID: machineID,
		sequence:  0,
		lastTime:  0,
	}, nil
}

// NextID generates the next unique ID
func (s *SnowflakeIDAllocator) NextID(ctx context.Context) (int64, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Get current timestamp in milliseconds
	now := time.Now().UnixMilli()

	if now < s.lastTime {
		return 0, fmt.Errorf("clock moved backwards, refusing to generate ID. lastTime: %d, now: %d", s.lastTime, now)
	}

	// Same millisecond: increment sequence
	if now == s.lastTime {
		// Check if sequence would overflow before incrementing
		if s.sequence >= maxSequence {
			// Sequence exhausted for this millisecond: wait for next millisecond
			now = s.waitNextMillis(now)
			s.sequence = 0 // Reset sequence for new millisecond
		} else {
			// Safe to increment sequence
			s.sequence = s.sequence + 1
		}
	} else {
		// New millisecond: reset sequence
		s.sequence = 0
	}

	s.lastTime = now

	// ID = (timestamp << timestampShift) | (machineID << machineIDShift) | sequence
	id := ((now - snowflakeEpoch) << timestampShift) |
		(s.machineID << machineIDShift) |
		s.sequence

	return id, nil
}

// waitNextMillis waits until the next millisecond
// Uses hybrid approach: sleep most of the way, then busy-wait for precision
func (s *SnowflakeIDAllocator) waitNextMillis(lastTime int64) int64 {
	now := time.Now().UnixMilli()
	maxWait := 10 // To prevent infinite loop

	for now <= lastTime && maxWait > 0 {
		// Calculate how long to wait
		remaining := lastTime - now + 1 // +1 to ensure we pass lastTime

		if remaining > 1 {
			// More than 1ms remaining then wait for next miliseconds
			time.Sleep(time.Duration(remaining-1) * time.Millisecond)
		}

		now = time.Now().UnixMilli()
		maxWait--
	}

	// if we still haven't advanced, force increment
	if now <= lastTime {
		now = lastTime + 1
	}

	return now
}

// GetMachineID returns the machine ID used by this allocator
func (s *SnowflakeIDAllocator) GetMachineID() int64 {
	return s.machineID
}

// DecodeSnowflakeID decodes a Snowflake ID into its components using bitwise operations
func DecodeSnowflakeID(id int64) (timestamp int64, machineID int64, sequence int64) {
	// Extract sequence (last 12 bits)
	sequence = id & maxSequence

	// Extract machine ID (bits 12-21): shift right then mask
	machineID = (id >> machineIDShift) & maxMachineID

	// Extract timestamp (bits 22-62): shift right to get timestamp part
	timestampPart := id >> timestampShift
	timestamp = timestampPart + snowflakeEpoch

	return timestamp, machineID, sequence
}
