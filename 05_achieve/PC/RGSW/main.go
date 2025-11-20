package main

import (
	"fmt"
	"math"
	"time"

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RGSW "github.com/CDSL-EncryptedControl/CDSL/utils/core/RGSW"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rgsw"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
)

func main() {
	// *****************************************************************
	// ************************* User's choice *************************
	// *****************************************************************
	// ============== Encryption parameters ==============
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN:    12,
		LogQ:    []int{56},
		LogP:    []int{51},
		NTTFlag: true,
	})
	fmt.Println("Degree of polynomials:", params.N())
	fmt.Println("Ciphertext modulus:", params.QBigInt())
	fmt.Println("Special modulus:", params.PBigInt())
	fmt.Println("Secret key distribution (Ternary):", params.Xs())
	fmt.Println("Error distribution (Discrete Gaussian):", params.Xe())

	// ============== Controller design (PID-based) ==============
	const (
		Kp = 34.0
		Ki = 4.0
		Kd = 42.0

		Lp = 40.0
		Li = 0.0
		Ld = 3.0
	)

	F := [][]float64{
		{1, 0, 0, 0},
		{0, 0, 0, 0},
		{0, 0, 1, 0},
		{0, 0, 0, 0},
	}

	G := [][]float64{
		{1, 0},
		{1, 0},
		{0, 1},
		{0, 1},
	}

	H := [][]float64{
		{Ki, -Kd, Li, -Ld},
	}

	R := [][]float64{
		{0, 0}, // Zero matrix
	}

	J := [][]float64{
		{Kp + Ki + Kd, Lp + Li + Ld},
	}

	// Controller initial state
	x_ini := []float64{0, 0, 0, 0}

	// Dimensions
	n := len(F)
	m := len(H)
	p := len(G[0])

	// ============== Quantization parameters ==============
	s := 1 / 1.0
	L := 1 / 100000.0
	r := 1 / 10000.0
	fmt.Printf("Scaling parameters 1/L: %v, 1/s: %v, 1/r: %v \n", 1/L, 1/s, 1/r)

	// ============== Encryption settings ==============
	levelQ := params.QCount() - 1
	levelP := params.PCount() - 1
	ringQ := params.RingQ()

	// Compute tau
	maxDim := math.Max(math.Max(float64(n), float64(m)), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

	// Generate DFS index for unpack
	dfsId := make([]int, tau)
	for i := 0; i < tau; i++ {
		dfsId[i] = i
	}
	tmp := make([]int, tau)
	for i := 1; i < tau; i *= 2 {
		id := 0
		currBlock := tau / i
		nextBlock := currBlock / 2
		for j := 0; j < i; j++ {
			for k := 0; k < nextBlock; k++ {
				tmp[id] = dfsId[j*currBlock+2*k]
				tmp[nextBlock+id] = dfsId[j*currBlock+2*k+1]
				id++
			}
			id += nextBlock
		}
		for j := 0; j < tau; j++ {
			dfsId[j] = tmp[j]
		}
	}

	// Generate monomials for unpack
	logn := int(math.Log2(float64(tau)))
	monomials := make([]ring.Poly, logn)
	for i := 0; i < logn; i++ {
		monomials[i] = ringQ.NewPoly()
		idx := params.N() - params.N()/(1<<(i+1))
		monomials[i].Coeffs[0][idx] = 1
		ringQ.MForm(monomials[i], monomials[i])
		ringQ.NTT(monomials[i], monomials[i])
	}

	// Generate Galois elements
	galEls := make([]uint64, int(math.Log2(float64(tau))))
	for i := 0; i < int(math.Log2(float64(tau))); i++ {
		galEls[i] = uint64(tau/int(math.Pow(2, float64(i))) + 1)
	}

	// Generate keys
	kgen := rlwe.NewKeyGenerator(params)
	sk := kgen.GenSecretKeyNew()
	rlk := kgen.GenRelinearizationKeyNew(sk)
	evkRGSW := rlwe.NewMemEvaluationKeySet(rlk)
	evkRLWE := rlwe.NewMemEvaluationKeySet(rlk, kgen.GenGaloisKeysNew(galEls, sk)...)

	encryptorRLWE := rlwe.NewEncryptor(params, sk)
	decryptorRLWE := rlwe.NewDecryptor(params, sk)
	encryptorRGSW := rgsw.NewEncryptor(params, sk)
	evaluatorRGSW := rgsw.NewEvaluator(params, evkRGSW)
	evaluatorRLWE := rlwe.NewEvaluator(params, evkRLWE)

	// ==============  Encryption of controller matrices ==============
	GBar := utils.ScalMatMult(1/s, G)
	HBar := utils.ScalMatMult(1/s, H)
	RBar := utils.ScalMatMult(1/s, R)
	JBar := utils.ScalMatMult(1/s, J)

	ctF := RGSW.EncPack(F, tau, encryptorRGSW, levelQ, levelP, ringQ, params)
	ctG := RGSW.EncPack(GBar, tau, encryptorRGSW, levelQ, levelP, ringQ, params)
	ctH := RGSW.EncPack(HBar, tau, encryptorRGSW, levelQ, levelP, ringQ, params)
	ctR := RGSW.EncPack(RBar, tau, encryptorRGSW, levelQ, levelP, ringQ, params)
	_ = ctR
	ctJ := RGSW.EncPack(JBar, tau, encryptorRGSW, levelQ, levelP, ringQ, params)

	// Zero ciphertext for safe additions
	zeroCt := rlwe.NewCiphertext(params, 1)

	// ============== Simulation ==============
	iter := 1000
	fmt.Printf("Number of iterations: %v\n", iter)

	// 1) Unencrypted controller
	yUnenc := [][]float64{}
	uUnenc := [][]float64{}
	xcUnenc := [][]float64{}

	x := x_ini
	for i := 0; i < iter; i++ {
		y := []float64{1.0, 0.0} // fixed step input
		u := utils.VecAdd(
			utils.MatVecMult(H, x),
			utils.MatVecMult(J, y),
		)
		x = utils.VecAdd(utils.MatVecMult(F, x), utils.MatVecMult(G, y))

		yUnenc = append(yUnenc, y)
		uUnenc = append(uUnenc, u)
		xcUnenc = append(xcUnenc, x)
	}

	// 2) Encrypted controller
	yEnc := [][]float64{}
	uEnc := [][]float64{}

	xBar := utils.RoundVec(utils.ScalVecMult(1/(r*s), x_ini))
	xCtPack := RLWE.EncPack(xBar, tau, 1/L, *encryptorRLWE, ringQ, params)

	period := make([][]float64, iter)
	startPeriod := make([]time.Time, iter)

	for i := 0; i < iter; i++ {
		y := []float64{1.0, 0.0}
		startPeriod[i] = time.Now()

		// Encrypt y
		yBar := utils.RoundVec(utils.ScalVecMult(1/r, y))
		yCtPack := RLWE.EncPack(yBar, tau, 1/L, *encryptorRLWE, ringQ, params)

		// Unpack state and input
		xCt := RLWE.UnpackCt(xCtPack, n, tau, evaluatorRLWE, ringQ, monomials, params)
		yCt := RLWE.UnpackCt(yCtPack, p, tau, evaluatorRLWE, ringQ, monomials, params)

		// Controller output: Hx + Jy
		uCtPack := RGSW.MultPack(xCt, ctH, evaluatorRGSW, ringQ, params)
		JyCt := RGSW.MultPack(yCt, ctJ, evaluatorRGSW, ringQ, params)
		uCtPack = RLWE.Add(uCtPack, JyCt, zeroCt, params)

		// Decrypt output
		u := RLWE.DecUnpack(uCtPack, m, tau, *decryptorRLWE, r*s*s*L, ringQ, params)

		// State update: x = F*x + G*y
		FxCt := RGSW.MultPack(xCt, ctF, evaluatorRGSW, ringQ, params)
		GyCt := RGSW.MultPack(yCt, ctG, evaluatorRGSW, ringQ, params)
		xCtPack = RLWE.Add(FxCt, GyCt, zeroCt, params)

		period[i] = []float64{float64(time.Since(startPeriod[i]).Nanoseconds()) / 1000000}

		yEnc = append(yEnc, y)
		uEnc = append(uEnc, u)
	}

	avgPeriod := utils.Average(utils.MatToVec(period))
	fmt.Println("Average elapsed time for a control period:", avgPeriod, "ms")

	// Compare unencrypted vs encrypted outputs
	uDiff := make([][]float64, iter)
	for i := range uDiff {
		uDiff[i] = []float64{utils.Vec2Norm(utils.VecSub(uUnenc[i], uEnc[i]))}
	}

	// =========== Export data ===========
	utils.DataExport(uUnenc, "./uUnenc.csv")
	utils.DataExport(uEnc, "./uEnc.csv")
	utils.DataExport(uDiff, "./uDiff.csv")
	utils.DataExport(period, "./period.csv")
}
