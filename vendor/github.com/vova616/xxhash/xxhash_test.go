//stollen from bitbucket.org/StephaneBunel/xxhash-go
package xxhash

import (
	"encoding/binary"
	"hash/adler32"
	"hash/crc32"
	"hash/fnv"
	"testing"
)

var (
	blob1       = []byte("Lorem ipsum dolor sit amet, consectetuer adipiscing elit, ")
	blob2       = []byte("sed diam nonummy nibh euismod tincidunt ut laoreet dolore magna aliquam erat volutpat.")
	blob3       = []byte("Cookies")
	blob4       = []byte("1234567890123456")
	VeryBigFile = "a-very-big-file"
)

func Test_Checksum32(t *testing.T) {
	h32 := Checksum32(blob1)
	if h32 != 0x1130e7d4 {
		t.Errorf("Checksum32(\"%v\") = 0x%08x need 0x1130e7d4\n", string(blob1), h32)
	}

	h32 = Checksum32(blob2)
	if h32 != 0x24ca2992 {
		t.Errorf("Checksum32(\"%v\") = 0x%08x need 0x24ca2992\n", string(blob2), h32)
	}

	h32 = Checksum32(blob3)
	if h32 != 0x99dd2ca5 {
		t.Errorf("Checksum32(\"%v\") = 0x%08x need 0x99dd2ca5\n", string(blob3), h32)
	}

	h32 = Checksum32(blob4)
	if h32 != 0x03bf5152 {
		t.Errorf("Checksum32(\"%v\") = 0x%08x need 0x03bf5152\n", string(blob4), h32)
	}
}

func Test_Checksum32Seed(t *testing.T) {
	h32 := Checksum32Seed(blob1, 1471)
	if h32 != 0xba59a258 {
		t.Errorf("Checksum32Seed(\"%v\", 1471) = 0x%08x\n need 0xba59a258", string(blob1), h32)
	}

	h32 = Checksum32Seed(blob2, 1596234)
	if h32 != 0xf15f3e02 {
		t.Errorf("Checksum32Seed(\"%v\", 1596234) = 0x%08x need 0xf15f3e02\n", string(blob2), h32)
	}

	h32 = Checksum32Seed(blob3, 9999666)
	if h32 != 0xcd3ae44c {
		t.Errorf("Checksum32Seed(\"%v\", 9999666) = 0x%08x need 0xcd3ae44c\n", string(blob3), h32)
	}

	h32 = Checksum32Seed(blob4, 1)
	if h32 != 0x606913c4 {
		t.Errorf("Checksum32Seed(\"%v\", 1) = 0x%08x need 0x606913c4\n", string(blob4), h32)
	}
}

func Test_New32(t *testing.T) {
	var digest = New(0)
	digest.Write(blob1)
	digest.Write(blob2)
	h32 := digest.Sum32()
	if h32 != 0x0d44373a {
		t.Errorf("Sum32 = 0x%08x need 0x0d44373a\n", h32)
	}

	digest = New(0)
	digest.Write(blob3)
	h32 = digest.Sum32()
	if h32 != 0x99dd2ca5 {
		t.Errorf("Sum32 = 0x%08x need 0x99dd2ca5\n", h32)
	}

	digest = New(0)
	digest.Write(blob4)
	h32 = digest.Sum32()
	if h32 != 0x3bf5152 {
		t.Errorf("Sum32 = 0x%08x need 0x3bf5152\n", h32)
	}
}

func Test_New32Seed(t *testing.T) {
	var digest = New(1471)
	digest.Write(blob1)
	digest.Write(blob2)
	h32 := digest.Sum32()
	if h32 != 0x3265e220 {
		t.Errorf("Sum32 = 0x%08x need 0x3265e220\n", h32)
	}

	digest = New(615324687)
	digest.Write(blob3)
	h32 = digest.Sum32()
	if h32 != 0xb90e95cb {
		t.Errorf("Sum32 = 0x%08x need 0x89f56371\n", h32)
	}

	digest = New(1)
	digest.Write(blob4)
	h32 = digest.Sum32()
	if h32 != 0x606913c4 {
		t.Errorf("Sum32 = 0x%08x need 0x606913c4\n", h32)
	}

}

func Test_Reset(t *testing.T) {
	var digest = New(0)
	digest.Write(blob2)
	digest.Reset()
	digest.Write(blob1)
	h32 := digest.Sum32()
	if h32 != 0x1130e7d4 {
		t.Errorf("Sum32 = 0x%08x need 0x1130e7d4\n", h32)
	}
}

func Benchmark_xxhash32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Checksum32(blob1)
	}
}

func Benchmark_CRC32IEEE(b *testing.B) {
	for i := 0; i < b.N; i++ {
		crc32.ChecksumIEEE(blob1)
	}
}

func Benchmark_Adler32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		adler32.Checksum(blob1)
	}
}

func Benchmark_Fnv32(b *testing.B) {
	h := fnv.New32()
	for i := 0; i < b.N; i++ {
		h.Sum(blob1)
	}
}

func Benchmark_MurmurHash3Hash32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		mmh3Hash32(blob1)
	}
}

// MurmurHash 3
// mmh3.Hash32 stollen from https://github.com/reusee/mmh3
func mmh3Hash32(key []byte) uint32 {
	length := len(key)
	if length == 0 {
		return 0
	}
	var c1, c2 uint32 = 0xcc9e2d51, 0x1b873593
	nblocks := length / 4
	var h, k uint32
	buf := key
	for i := 0; i < nblocks; i++ {
		k = binary.LittleEndian.Uint32(buf)
		buf = buf[4:]
		k *= c1
		k = (k << 15) | (k >> (32 - 15))
		k *= c2
		h ^= k
		h = (h << 13) | (h >> (32 - 13))
		h = (h * 5) + 0xe6546b64
	}
	k = 0
	tailIndex := nblocks * 4
	switch length & 3 {
	case 3:
		k ^= uint32(key[tailIndex+2]) << 16
		fallthrough
	case 2:
		k ^= uint32(key[tailIndex+1]) << 8
		fallthrough
	case 1:
		k ^= uint32(key[tailIndex])
		k *= c1
		k = (k << 13) | (k >> (32 - 15))
		k *= c2
		h ^= k
	}
	h ^= uint32(length)
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}
