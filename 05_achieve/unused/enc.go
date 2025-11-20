package main

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

func main() {
	slots := 10

	// Generate plaintext modulus
	logN := 12
	primeGen := ring.NewNTTFriendlyPrimesGenerator(18, uint64(math.Pow(2, float64(logN))))
	prime, _ := primeGen.NextAlternatingPrime()
	fmt.Println("Plaintext modulus:", prime)

	// Plaintext messages
	m1 := make([]uint64, slots)
	m2 := make([]uint64, slots)
	m1plusm2 := make([]uint64, slots)
	m1prodm2 := make([]float64, slots)
	m1InnerSum := uint64(0)

	for i := 0; i < slots; i++ {
		m1[i] = uint64(rand.Intn(1000))
		m2[i] = uint64(rand.Intn(1000))
		m1plusm2[i] = m1[i] + m2[i]
		m1prodm2[i] = math.Mod(float64(m1[i]*m2[i]), float64(prime))
		m1InnerSum = m1InnerSum + m1[i]
	}

	fmt.Println("message 1:", m1)
	fmt.Println("message 2:", m2)

	// 128-bit secure BGV parameters
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             []int{28, 28}, // 128bit security 맞추기 위해 Q가 줄어듦 QP에 맞춰야대서
		LogP:             []int{15},     // special modulus (P=우리가 아는 1/L)
		PlaintextModulus: prime,
	})

	fmt.Println("log2 plaintext modulus:", "around", math.Round(params.LogT()))
	// level = number of possible multiplications (mostly)
	fmt.Println("maximum level:", params.MaxLevel())
	// actual ciphertext modulus = QP
	fmt.Println("ciphertext modulus:", params.QPBigInt(), "around 2^", math.Round(params.LogQP()))

	// Key Generator
	kgen := rlwe.NewKeyGenerator(params)

	// Secret Key
	sk := kgen.GenSecretKeyNew()

	// Encoder
	ecd := bgv.NewEncoder(params)

	// Encryptor
	enc := rlwe.NewEncryptor(params, sk)

	// Decryptor
	dec := rlwe.NewDecryptor(params, sk)

	// Create empty plaintexts
	pt1 := bgv.NewPlaintext(params, params.MaxLevel())
	pt2 := bgv.NewPlaintext(params, params.MaxLevel())

	// NTT packing
	ecd.Encode(m1, pt1)
	ecd.Encode(m2, pt2)

	// Encryption
	ct1, _ := enc.EncryptNew(pt1)
	ct2, _ := enc.EncryptNew(pt2)

	// degree = (number of polynomials in a ciphertext) - 1
	fmt.Println("degree of newly encrypted ciphertext:", ct1.Degree())

	// empty slots for decrypted messages
	m1dec := make([]uint64, slots)
	m2dec := make([]uint64, slots)

	// Decryption
	ct1dec := dec.DecryptNew(ct1)
	ct2dec := dec.DecryptNew(ct2)

	// NTT unpacking
	ecd.Decode(ct1dec, m1dec)
	ecd.Decode(ct2dec, m2dec)

	fmt.Println("decrypted message 1:", m1dec)
	fmt.Println("decrypted message 2:", m2dec)

	// Generate evaluation keys
	// Relinearization key
	rlk := kgen.GenRelinearizationKeyNew(sk)
	// For rotation
	// e.g. rot = 1: shift 1 to left, rot = -1: shift 1 to right
	rot := int(1)
	// obtain Galois elements for rotation, generate Galois key
	gk := kgen.GenGaloisKeyNew(params.GaloisElementForColRotation(rot), sk)

	// Evaluation key containing rlk, gk
	evk := rlwe.NewMemEvaluationKeySet(rlk, gk)

	// Evaluator
	eval := bgv.NewEvaluator(params, evk)
	// eval_1 := bgv.NewEvaluator(params)
	// eval_2 := bgv.NewEvaluator(params)

	// Homomorphic addition
	ctAdd, _ := eval.AddNew(ct1, ct2)

	ctAddDec := dec.DecryptNew(ctAdd)

	addDec := make([]uint64, slots)
	ecd.Decode(ctAddDec, addDec)

	fmt.Println("m1+m2:", m1plusm2)
	fmt.Println("homomorphic addition:", addDec)

	// Homomorphic multiplication
	ctMult, _ := eval.MulNew(ct1, ct2)

	fmt.Println("degree after multplication:", ctMult.Degree())

	fmt.Println("m1*m2:", m1prodm2)

	ctMultDec := dec.DecryptNew(ctMult)
	multDec := make([]uint64, slots)
	ecd.Decode(ctMultDec, multDec)

	fmt.Println("homomorphic multiplication:", multDec)

	// Relinearization
	ctMultRelin, _ := eval.RelinearizeNew(ctMult)

	fmt.Println("degree after relinearization:", ctMultRelin.Degree())

	ctMultRelinDec := dec.DecryptNew(ctMultRelin)

	relinDec := make([]uint64, slots)
	ecd.Decode(ctMultRelinDec, relinDec)

	fmt.Println("multiplication+relinearization:", relinDec)

	// Rotation
	ctRotate, _ := eval.RotateColumnsNew(ct1, rot)
	ctRotateDec := dec.DecryptNew(ctRotate)
	rotDec := make([]uint64, params.N())
	ecd.Decode(ctRotateDec, rotDec)

	fmt.Println("rotation:", rotDec)
	fmt.Println("Galois element:", params.GaloisElementForColRotation(rot))

	// Inner sum (add all elements within vector)
	ctInSum := rlwe.NewCiphertext(params, 1, params.MaxLevel())
	// Obtain Galois elements for inner sum
	gkInSum := kgen.GenGaloisKeysNew(params.GaloisElementsForInnerSum(1, slots), sk)
	// New evaluation key with gkInSum
	evkInSum := rlwe.NewMemEvaluationKeySet(rlk, gkInSum...)
	// New evaluator from evkInSum
	evalInSum := bgv.NewEvaluator(params, evkInSum)
	evalInSum.InnerSum(ct1, 1, slots, ctInSum)
	ctInSumDec := dec.DecryptNew(ctInSum)
	insumDec := make([]uint64, slots)
	ecd.Decode(ctInSumDec, insumDec)

	fmt.Println("inner sum:", insumDec)
	fmt.Println("Galois elements for inner sum:", params.GaloisElementsForInnerSum(1, slots))
	fmt.Println("m1 elements sum:", m1InnerSum)
}
