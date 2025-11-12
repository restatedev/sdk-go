package rand

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/google/uuid"
)

type Rand interface {
	// Deprecated: Use restate.UUID directly, instead of restate.Rand().UUID()
	UUID() uuid.UUID
	Float64() float64
	Uint64() uint64
	// Source returns a deterministic random source that can be provided to math/rand.New()
	// and math/rand/v2.New(). The v2 version of rand is strongly recommended where Go 1.22
	// is used, and once this library begins to depend on 1.22, it will be embedded in Rand.
	//
	// Deprecated: Use restate.RandSource directly, instead of restate.Rand().Source()
	Source() Source
}

type rand struct {
	source *source
}

func NewFromInvocationId(invocationID []byte) *rand {
	return &rand{newSourceFromInvocationId(invocationID)}
}

func NewFromSeed(seed uint64) *rand {
	return &rand{newSource(seed)}
}

func (r *rand) UUID() uuid.UUID {
	var uuid [16]byte
	binary.LittleEndian.PutUint64(uuid[:8], r.Uint64())
	binary.LittleEndian.PutUint64(uuid[8:], r.Uint64())
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10
	return uuid
}

func (r *rand) Float64() float64 {
	// use the math/rand/v2 implementation of Float64() which is more correct
	// and also matches our TS implementation
	return float64(r.Uint64()<<11>>11) / (1 << 53)
}

func (r *rand) Uint64() uint64 {
	return r.source.Uint64()
}

func (r *rand) Source() Source {
	return r.source
}

type Source interface {
	Int63() int64
	Uint64() uint64

	// only the v1 rand package requires this method; we do *not* support it and will panic if its called.
	Seed(int64)
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

func (s *source) Int63() int64 {
	return int64(s.Uint64() & ((1 << 63) - 1))
}

// only the v1 rand package has this method
func (s *source) Seed(int64) {
	panic("The Restate random source is already deterministic based on invocation ID and must not be seeded")
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
