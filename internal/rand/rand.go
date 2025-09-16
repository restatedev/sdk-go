package rand

import (
	"crypto/sha256"
	"encoding/binary"

	stdrand "math/rand/v2"

	"github.com/google/uuid"
)

type Rand struct {
	stdrand.Rand
}

func NewFromSeed(seed uint64) *Rand {
	r := stdrand.New(newSource(seed))

	return &Rand{*r}
}

func NewFromInvocationId(invocationId []byte) *Rand {
	r := stdrand.New(newSourceFromInvocationId(invocationId))

	return &Rand{*r}
}

func (r *Rand) UUID() uuid.UUID {
	var uuid [16]byte
	binary.LittleEndian.PutUint64(uuid[:8], r.Uint64())
	binary.LittleEndian.PutUint64(uuid[8:], r.Uint64())
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10
	return uuid
}

type source struct {
	state [4]uint64
}

// From https://xoroshiro.di.unimi.it/splitmix64.c
func splitMix64(x *uint64) uint64 {
	*x += 0x9e3779b97f4a7c15
	z := *x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func newSource(seed uint64) *source {
	return &source{state: [4]uint64{
		splitMix64(&seed),
		splitMix64(&seed),
		splitMix64(&seed),
		splitMix64(&seed),
	}}
}

func newSourceFromInvocationId(invocationId []byte) *source {
	hash := sha256.New()
	hash.Write(invocationId)
	var sum [32]byte
	hash.Sum(sum[:0])

	return &source{state: [4]uint64{
		binary.LittleEndian.Uint64(sum[:8]),
		binary.LittleEndian.Uint64(sum[8:16]),
		binary.LittleEndian.Uint64(sum[16:24]),
		binary.LittleEndian.Uint64(sum[24:32]),
	}}
}

func (s *source) Uint64() uint64 {
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
