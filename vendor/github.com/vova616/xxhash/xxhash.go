package xxhash

import (
	"errors"
	"hash"
	"unsafe"
)

const (
	PRIME32_1 = 2654435761
	PRIME32_2 = 2246822519
	PRIME32_3 = 3266489917
	PRIME32_4 = 668265263
	PRIME32_5 = 374761393
)

type XXHash struct {
	seed, v1, v2, v3, v4 uint32
	total_len            uint64
	memory               [16]byte
	memsize              int
}

func New(seed uint32) hash.Hash32 {
	return &XXHash{
		seed: seed,
		v1:   seed + PRIME32_1 + PRIME32_2,
		v2:   seed + PRIME32_2,
		v3:   seed,
		v4:   seed - PRIME32_1,
	}
}

func (self *XXHash) BlockSize() int {
	return 1
}

// Size returns the number of bytes Sum will return.
func (self *XXHash) Size() int {
	return 4
}

func (self *XXHash) feed(in []byte) uint32 {
	p := uintptr(unsafe.Pointer(&in[0]))
	pTemp := p
	l := len(in)
	bEnd := p + uintptr(l)

	self.total_len += uint64(l)

	// fill in tmp buffer
	if self.memsize+l < 16 {
		copy(self.memory[self.memsize:], in)
		self.memsize += l
		return 0
	}

	if self.memsize > 0 {
		copy(self.memory[self.memsize:], in[:16-self.memsize])
		p2 := uintptr(unsafe.Pointer(&self.memory[0]))
		self.v1 += (*(*uint32)(unsafe.Pointer(p2))) * PRIME32_2
		self.v1 = ((self.v1 << 13) | (self.v1 >> (32 - 13))) * PRIME32_1

		self.v2 += (*(*uint32)(unsafe.Pointer(p2 + 4))) * PRIME32_2
		self.v2 = ((self.v2 << 13) | (self.v2 >> (32 - 13))) * PRIME32_1

		self.v3 += (*(*uint32)(unsafe.Pointer(p2 + 8))) * PRIME32_2
		self.v3 = ((self.v3 << 13) | (self.v3 >> (32 - 13))) * PRIME32_1

		self.v4 += (*(*uint32)(unsafe.Pointer(p2 + 12))) * PRIME32_2
		self.v4 = ((self.v4 << 13) | (self.v4 >> (32 - 13))) * PRIME32_1

		p += 16 - uintptr(self.memsize)
		self.memsize = 0
	}

	limit := bEnd - 16
	v1, v2, v3, v4 := self.v1, self.v2, self.v3, self.v4

	for ; p <= limit; p += 16 {
		v1 += (*(*uint32)(unsafe.Pointer(p))) * PRIME32_2
		v1 = ((v1 << 13) | (v1 >> (32 - 13))) * PRIME32_1

		v2 += (*(*uint32)(unsafe.Pointer(p + 4))) * PRIME32_2
		v2 = ((v2 << 13) | (v2 >> (32 - 13))) * PRIME32_1

		v3 += (*(*uint32)(unsafe.Pointer(p + 8))) * PRIME32_2
		v3 = ((v3 << 13) | (v3 >> (32 - 13))) * PRIME32_1

		v4 += (*(*uint32)(unsafe.Pointer(p + 12))) * PRIME32_2
		v4 = ((v4 << 13) | (v4 >> (32 - 13))) * PRIME32_1
	}

	self.v1 = v1
	self.v2 = v2
	self.v3 = v3
	self.v4 = v4

	limit = bEnd - p

	if limit > 0 {
		copy(self.memory[:], in[p-pTemp:bEnd-pTemp])
		self.memsize = int(limit)
	}

	return 0
}

func (self *XXHash) Sum32() uint32 {
	p := uintptr(unsafe.Pointer(&self.memory[0]))
	bEnd := p + uintptr(self.memsize)
	h32 := uint32(0)

	if self.total_len >= 16 {
		h32 = ((self.v1 << 1) | (self.v1 >> (32 - 1))) +
			((self.v2 << 7) | (self.v2 >> (32 - 7))) +
			((self.v3 << 12) | (self.v3 >> (32 - 12))) +
			((self.v4 << 18) | (self.v4 >> (32 - 18)))
	} else {
		h32 = self.seed + PRIME32_5
	}

	h32 += uint32(self.total_len)

	for p <= bEnd-4 {
		h32 += (*(*uint32)(unsafe.Pointer(p))) * PRIME32_3
		h32 = ((h32 << 17) | (h32 >> (32 - 17))) * PRIME32_4
		p += 4
	}

	for p < bEnd {
		h32 += uint32(*(*byte)(unsafe.Pointer(p))) * PRIME32_5
		h32 = ((h32 << 11) | (h32 >> (32 - 11))) * PRIME32_1
		p++
	}

	h32 ^= h32 >> 15
	h32 *= PRIME32_2
	h32 ^= h32 >> 13
	h32 *= PRIME32_3
	h32 ^= h32 >> 16

	return h32
}

func (self *XXHash) Sum(in []byte) []byte {
	h := self.Sum32()
	in = append(in, byte(h>>24))
	in = append(in, byte(h>>16))
	in = append(in, byte(h>>8))
	in = append(in, byte(h))
	return in
}

func (self *XXHash) Reset() {
	seed := self.seed
	self.v1 = seed + PRIME32_1 + PRIME32_2
	self.v2 = seed + PRIME32_2
	self.v3 = seed
	self.v4 = seed - PRIME32_1
	self.total_len = 0
	self.memsize = 0
}

// Write adds more data to the running hash.
// Length of data MUST BE less than 1 Gigabytes.
func (self *XXHash) Write(data []byte) (nn int, err error) {
	if data == nil {
		return 0, errors.New("Data cannot be nil.")
	}
	l := len(data)
	if l > 1<<30 {
		return 0, errors.New("Cannot add more than 1 Gigabytes at once.")
	}
	self.feed(data)
	return len(data), nil
}

// Checksum32Seed returns the xxhash32 checksum of data using a seed. Length of data MUST BE less than 2 Gigabytes.
func Checksum32(data []byte) uint32 {
	return Checksum32Seed(data, 0)
}

// Checksum32 returns the xxhash32 checksum of data. Length of data MUST BE less than 2 Gigabytes.
func Checksum32Seed(data []byte, seed uint32) uint32 {
	if data == nil {
		panic("Data cannot be nil.")
	}
	p := uintptr(unsafe.Pointer(&data[0]))
	l := len(data)
	bEnd := p + uintptr(l)
	h32 := uint32(0)

	if l >= 16 {
		limit := bEnd - 16

		v1 := seed + PRIME32_1 + PRIME32_2
		v2 := seed + PRIME32_2
		v3 := seed + 0
		v4 := seed - PRIME32_1
		for {
			v1 += (*(*uint32)(unsafe.Pointer(p))) * PRIME32_2
			v1 = ((v1 << 13) | (v1 >> (32 - 13))) * PRIME32_1

			v2 += (*(*uint32)(unsafe.Pointer(p + 4))) * PRIME32_2
			v2 = ((v2 << 13) | (v2 >> (32 - 13))) * PRIME32_1

			v3 += (*(*uint32)(unsafe.Pointer(p + 8))) * PRIME32_2
			v3 = ((v3 << 13) | (v3 >> (32 - 13))) * PRIME32_1

			v4 += (*(*uint32)(unsafe.Pointer(p + 12))) * PRIME32_2
			v4 = ((v4 << 13) | (v4 >> (32 - 13))) * PRIME32_1

			p += 16
			if p > limit {
				break
			}
		}
		h32 = ((v1 << 1) | (v1 >> (32 - 1))) +
			((v2 << 7) | (v2 >> (32 - 7))) +
			((v3 << 12) | (v3 >> (32 - 12))) +
			((v4 << 18) | (v4 >> (32 - 18)))
	} else {
		h32 = seed + PRIME32_5
	}

	h32 += uint32(l)

	for p <= bEnd-4 {
		h32 += (*(*uint32)(unsafe.Pointer(p))) * PRIME32_3
		h32 = ((h32 << 17) | (h32 >> (32 - 17))) * PRIME32_4
		p += 4
	}

	for p < bEnd {
		h32 += uint32(*(*byte)(unsafe.Pointer(p))) * PRIME32_5
		h32 = ((h32 << 11) | (h32 >> (32 - 11))) * PRIME32_1
		p++
	}

	h32 ^= h32 >> 15
	h32 *= PRIME32_2
	h32 ^= h32 >> 13
	h32 *= PRIME32_3
	h32 ^= h32 >> 16

	return h32
}
