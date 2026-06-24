package randsource

import (
	"crypto/sha256"
	"encoding/binary"
)

type Source struct {
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

func NewFromSeed(seed uint64) *Source {
	return &Source{state: [4]uint64{
		splitMix64(&seed),
		splitMix64(&seed),
		splitMix64(&seed),
		splitMix64(&seed),
	}}
}

func NewFromInvocationId(invocationId []byte) *Source {
	hash := sha256.New()
	hash.Write(invocationId)
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

func (s *Source) Copy() *Source {
	return &Source{state: [4]uint64{
		s.state[0],
		s.state[1],
		s.state[2],
		s.state[3],
	}}
}

func rotl(x uint64, k uint64) uint64 {
	return (x << k) | (x >> (64 - k))
}
