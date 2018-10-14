package common

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	mRand "math/rand"
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
