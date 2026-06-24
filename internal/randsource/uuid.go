package randsource

import (
	"encoding/binary"
	rand2 "math/rand/v2"

	"github.com/google/uuid"
)

func UUIDFromRand(rand *rand2.Rand) uuid.UUID {
	var bytes [16]byte
	binary.LittleEndian.PutUint64(bytes[:8], rand.Uint64())
	binary.LittleEndian.PutUint64(bytes[8:], rand.Uint64())
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // Version 4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // Variant is 10
	return bytes
}
