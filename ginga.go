package ginga

import (
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"math/bits"
)

// Constantes principais da cifra
const BlockSize = 16
const Rounds = 16

// --- Funções auxiliares ARX ---

func add32(x, y uint32) uint32      { return x + y }
func sub32(x, y uint32) uint32      { return x - y }
func rotl32(x uint32, n int) uint32 { return bits.RotateLeft32(x, n) }
func rotr32(x uint32, n int) uint32 { return bits.RotateLeft32(x, -n) }

func confuse32(x uint32) uint32 {
	x ^= 0xA5A5A5A5  // 1. XOR
	x += 0x3C3C3C3C  // 2. ADD
	x = rotl32(x, 7) // 3. ROTATE LEFT
	return x
}

func deconfuse32(x uint32) uint32 {
	x = rotr32(x, 7) // 1. ROTATE RIGHT (inverse of rotl)
	x -= 0x3C3C3C3C  // 2. SUB (inverse of add)
	x ^= 0xA5A5A5A5  // 3. XOR (same as XOR inverse)
	return x
}

func round32(x, k uint32, r int) uint32 {
	x = add32(x, k)
	x = confuse32(x)
	x = rotl32(x, (r+3)&31)
	x ^= k
	x = rotl32(x, (r+5)&31)
	return x
}

func invRound32(x, k uint32, r int) uint32 {
	x = rotr32(x, (r+5)&31)
	x ^= k
	x = rotr32(x, (r+3)&31)
	x = deconfuse32(x)
	x = sub32(x, k)
	return x
}

func subKey32(k *[8]uint32, round, i int) uint32 {
	base := k[(i+round)&7]
	return rotl32(base^uint32(i*73+round*91), (round+i)&31)
}

func mixState32(state *[4]uint32) {
	state[0] ^= rotl32(state[1], 5)
	state[1] ^= rotl32(state[2], 11)
	state[2] ^= rotl32(state[3], 17)
	state[3] ^= rotl32(state[0], 23)
}

func invMixState32(state *[4]uint32) {
	state[3] ^= rotl32(state[0], 23)
	state[2] ^= rotl32(state[3], 17)
	state[1] ^= rotl32(state[2], 11)
	state[0] ^= rotl32(state[1], 5)
}

// --- Encrypt/Decrypt ---

func Encrypt(plain, key []byte) ([]byte, error) {
	if len(plain) != BlockSize {
		return nil, errors.New("ginga: plaintext must be 16 bytes")
	}
	if len(key) != 32 {
		return nil, errors.New("ginga: key must be 32 bytes (256 bits)")
	}

	var c [4]uint32
	for i := 0; i < 4; i++ {
		c[i] = binary.LittleEndian.Uint32(plain[i*4 : (i+1)*4])
	}

	var k [8]uint32
	for i := 0; i < 8; i++ {
		k[i] = binary.LittleEndian.Uint32(key[i*4 : (i+1)*4])
	}

	for r := 0; r < Rounds; r++ {
		for i := 0; i < 4; i++ {
			subk := subKey32(&k, r, i)
			c[i] = round32(c[i], subk, r)
		}
		mixState32(&c)
	}

	out := make([]byte, BlockSize)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(out[i*4:(i+1)*4], c[i])
	}
	return out, nil
}

func Decrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) != BlockSize {
		return nil, errors.New("ginga: ciphertext must be 16 bytes")
	}
	if len(key) != 32 {
		return nil, errors.New("ginga: key must be 32 bytes (256 bits)")
	}

	var p [4]uint32
	for i := 0; i < 4; i++ {
		p[i] = binary.LittleEndian.Uint32(ciphertext[i*4 : (i+1)*4])
	}

	var k [8]uint32
	for i := 0; i < 8; i++ {
		k[i] = binary.LittleEndian.Uint32(key[i*4 : (i+1)*4])
	}

	for r := Rounds - 1; r >= 0; r-- {
		invMixState32(&p)
		for i := 0; i < 4; i++ {
			subk := subKey32(&k, r, i)
			p[i] = invRound32(p[i], subk, r)
		}
	}

	out := make([]byte, BlockSize)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(out[i*4:(i+1)*4], p[i])
	}
	return out, nil
}

// --- Integração com cipher.Block (NewCipher) ---

type gingaCipher struct {
	key []byte
}

// NewCipher cria um objeto cipher.Block compatível com modos de operação
func NewCipher(key []byte) (cipher.Block, error) {
	if len(key) != 32 {
		return nil, errors.New("ginga: invalid key size (must be 32 bytes)")
	}
	return &gingaCipher{key: append([]byte(nil), key...)}, nil
}

// BlockSize retorna o tamanho do bloco da cifra (16 bytes)
func (c *gingaCipher) BlockSize() int {
	return BlockSize
}

// Encrypt cifra exatamente um bloco de 16 bytes
func (c *gingaCipher) Encrypt(dst, src []byte) {
	if len(src) < BlockSize || len(dst) < BlockSize {
		panic("ginga: input not full block")
	}
	out, err := Encrypt(src[:BlockSize], c.key)
	if err != nil {
		panic("ginga: encryption failed: " + err.Error())
	}
	copy(dst, out)
}

// Decrypt decifra exatamente um bloco de 16 bytes
func (c *gingaCipher) Decrypt(dst, src []byte) {
	if len(src) < BlockSize || len(dst) < BlockSize {
		panic("ginga: input not full block")
	}
	out, err := Decrypt(src[:BlockSize], c.key)
	if err != nil {
		panic("ginga: decryption failed: " + err.Error())
	}
	copy(dst, out)
}
