package common

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	mRand "math/rand"
	"path/filepath"
	"time"
)

func init() {
	mRand.Seed(time.Now().UnixNano())
}

// Returns a cryptographically secure pseudorandom int in the interval [0;max)
func RandomInt(max int) int {
	i, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if i == nil {
		return mRand.Int() % max // Fallback
	}
	return int(i.Int64())
}

// Decode hex string to MD5 hash
func DecodeMD5(s string) (buf [16]byte, err error) {
	n, err := hex.Decode(buf[:], []byte(s))
	if err != nil {
		return
	}
	if n != 16 {
		err = Error{fmt.Errorf("invalid MD5 hash: `%s`", err)}
	}
	return
}

// Return filesystem path to thumbnail file
func ThumbPath(md5 [16]byte) string {
	return filepath.Join(RootDir, "images", hex.EncodeToString(md5[:]))
}

// Source of cryptographically secure integers
var CryptoSource mRand.Source = new(cryptoSource)

type cryptoSource struct{}

func (cryptoSource) Int63() int64 {
	var b [8]byte
	rand.Read(b[:])
	// Mask off sign bit to ensure positive number
	return int64(binary.LittleEndian.Uint64(b[:]) & (1<<63 - 1))
}

func (cryptoSource) Seed(_ int64) {}
