package rand

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"

	"github.com/google/uuid"
)

type Rand struct {
	*rand.Rand
}

func New(invocationID []byte) *Rand {
	return &Rand{rand.New(newSource(invocationID))}
}

func (r *Rand) UUID() uuid.UUID {
	var uuid [16]byte
	binary.LittleEndian.PutUint64(uuid[:8], r.Uint64())
	binary.LittleEndian.PutUint64(uuid[8:], r.Uint64())
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10
	return uuid
}

type Source struct {
	state [4]uint64
}

func newSource(invocationID []byte) *Source {
	hash := sha256.New()
	hash.Write(invocationID)
	var sum [32]byte
	hash.Sum(sum[:0])

	return &Source{state: [4]uint64{
		binary.LittleEndian.Uint64(sum[:8]),
		binary.LittleEndian.Uint64(sum[8:16]),
		binary.LittleEndian.Uint64(sum[16:24]),
		binary.LittleEndian.Uint64(sum[24:32]),
	}}
}

func (s *Source) Uint64() uint64 {
	result := rotl((s.state[0]+s.state[3]), 23) + s.state[0]

	t := (s.state[1] << 17)

	s.state[2] ^= s.state[0]
	s.state[3] ^= s.state[1]
	s.state[1] ^= s.state[2]
	s.state[0] ^= s.state[3]

	s.state[2] ^= t

	s.state[3] = rotl(s.state[3], 45)

	return result
}

func rotl(x uint64, k uint64) uint64 {
	return (x << k) | (x >> (64 - k))
}

var _ rand.Source = (*Source)(nil)
