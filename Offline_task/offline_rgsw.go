package main

import (
	"Encrypted_Cartpole/com_utils"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"time"

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RGSW "github.com/CDSL-EncryptedControl/CDSL/utils/core/RGSW"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rgsw"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
)

func main() {
	// ================= Encryption parameters =================
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

	// ================= Controller (PID-based) =================
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
		{0, 0},
	}
	_ = R
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
	s := 1 / 10000.0
	L := 1 / 1000.0
	r := 1 / 10000.0
	fmt.Printf("Scaling parameters 1/L: %v, 1/s: %v, 1/r: %v \n", 1/L, 1/s, 1/r)

	// ============== Common rings/aux ==============
	ringQ := params.RingQ()

	// Compute tau
	maxDim := math.Max(math.Max(float64(n), float64(m)), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

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

	// Zero ciphertext for safe additions
	zeroCt := rlwe.NewCiphertext(params, 1)

	// ============== Paths ==============
	base := filepath.Join("enc_data", "rgsw")

	// ============== 2) LOAD artifacts as recovered_* ==============
	recoveredX := new(rlwe.Ciphertext)
	if err := com_utils.ReadRT(filepath.Join(base, "xCtPack.dat"), recoveredX); err != nil {
		log.Fatalf("load xCtPack failed: %v", err)
	}

	recoveredF, err := com_utils.LoadRGSWPack(base, "ctF")
	if err != nil {
		log.Fatalf("load ctF failed: %v", err)
	}
	recoveredG, err := com_utils.LoadRGSWPack(base, "ctG")
	if err != nil {
		log.Fatalf("load ctG failed: %v", err)
	}
	recoveredH, err := com_utils.LoadRGSWPack(base, "ctH")
	if err != nil {
		log.Fatalf("load ctH failed: %v", err)
	}
	// R은 사용 안 하므로 로드만 하고 버려도 됨
	_, _ = com_utils.LoadRGSWPack(base, "ctR")

	recoveredJ, err := com_utils.LoadRGSWPack(base, "ctJ")
	if err != nil {
		log.Fatalf("load ctJ failed: %v", err)
	}

	recoveredRlk := new(rlwe.RelinearizationKey)
	if err := com_utils.ReadRT(filepath.Join(base, "rlk.dat"), recoveredRlk); err != nil {
		log.Fatalf("load rlk failed: %v", err)
	}
	recoveredGks, err := com_utils.LoadGaloisKeys(base)
	if err != nil {
		log.Fatalf("load gk_* failed: %v", err)
	}
	recoveredSk := new(rlwe.SecretKey)
	if err := com_utils.ReadRT(filepath.Join(base, "sk.dat"), recoveredSk); err != nil {
		log.Fatalf("load sk failed: %v", err)
	}

	// ============== Build evaluators/cryptors from recovered keys ==============
	evkRGSW2 := rlwe.NewMemEvaluationKeySet(recoveredRlk)
	evkRLWE2 := rlwe.NewMemEvaluationKeySet(recoveredRlk, recoveredGks...)
	evaluatorRGSW2 := rgsw.NewEvaluator(params, evkRGSW2)
	evaluatorRLWE2 := rlwe.NewEvaluator(params, evkRLWE2)
	encryptorRLWE2 := rlwe.NewEncryptor(params, recoveredSk)
	decryptorRLWE2 := rlwe.NewDecryptor(params, recoveredSk)

	// ============== Simulation setup ==============
	iter := 1000
	fmt.Printf("Number of iterations: %v\n", iter)

	// -------- Unencrypted baseline (for uDiff) --------
	yUnenc := [][]float64{}
	uUnenc := [][]float64{}
	xcUnenc := [][]float64{}

	x := append([]float64(nil), x_ini...) // copy
	for i := 0; i < iter; i++ {
		y := []float64{0.001, 0.001}
		u := utils.VecAdd(
			utils.MatVecMult(H, x),
			utils.MatVecMult(J, y),
		)
		x = utils.VecAdd(utils.MatVecMult(F, x), utils.MatVecMult(G, y))

		yUnenc = append(yUnenc, y)
		uUnenc = append(uUnenc, u)
		xcUnenc = append(xcUnenc, x)
	}

	// -------- Encrypted loop with recovered objects --------
	yEnc := [][]float64{}
	uEnc := [][]float64{}
	period := make([][]float64, iter)
	startPeriod := make([]time.Time, iter)

	// start from recovered packed state
	xCtPack := recoveredX

	for i := 0; i < iter; i++ {
		y := []float64{0.001, 0.001}
		startPeriod[i] = time.Now()

		// Encrypt y with recovered secret key
		yBar := utils.RoundVec(utils.ScalVecMult(1/r, y))
		yCtPack := RLWE.EncPack(yBar, tau, 1/L, *encryptorRLWE2, ringQ, params)

		// Unpack state and input
		xCt := RLWE.UnpackCt(xCtPack, n, tau, evaluatorRLWE2, ringQ, monomials, params)
		yCt := RLWE.UnpackCt(yCtPack, p, tau, evaluatorRLWE2, ringQ, monomials, params)

		// Controller output: u = Hx + Jy  (use recoveredH, recoveredJ)
		uCtPack := RGSW.MultPack(xCt, recoveredH, evaluatorRGSW2, ringQ, params)
		JyCt := RGSW.MultPack(yCt, recoveredJ, evaluatorRGSW2, ringQ, params)
		uCtPack = RLWE.Add(uCtPack, JyCt, zeroCt, params)

		// Decrypt output
		u := RLWE.DecUnpack(uCtPack, m, tau, *decryptorRLWE2, r*s*s*L, ringQ, params)

		// State update: x = F*x + G*y  (use recoveredF, recoveredG)
		FxCt := RGSW.MultPack(xCt, recoveredF, evaluatorRGSW2, ringQ, params)
		GyCt := RGSW.MultPack(yCt, recoveredG, evaluatorRGSW2, ringQ, params)
		xCtPack = RLWE.Add(FxCt, GyCt, zeroCt, params)

		period[i] = []float64{float64(time.Since(startPeriod[i]).Nanoseconds()) / 1e6}

		yEnc = append(yEnc, y)
		uEnc = append(uEnc, u)
	}

	avgPeriod := utils.Average(utils.MatToVec(period))
	fmt.Println("Average elapsed time for a control period (recovered):", avgPeriod, "ms")

	// -------- Compare unencrypted vs encrypted outputs --------
	uDiff := make([][]float64, iter)
	for i := range uDiff {
		uDiff[i] = []float64{utils.Vec2Norm(utils.VecSub(uUnenc[i], uEnc[i]))}
	}

	// 필요 시 CSV로 내보내기
	// utils.DataExport(uUnenc, "./uUnenc.csv")
	// utils.DataExport(uEnc, "./uEnc.csv")
	// utils.DataExport(uDiff, "./uDiff.csv")
	// utils.DataExport(period, "./period.csv")
}
