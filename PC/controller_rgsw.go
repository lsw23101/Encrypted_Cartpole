package main

import (
	"Encrypted_Cartpole/com_utils"
	"bufio"
	"fmt"
	"log"
	"math"
	"net"
	"path/filepath"
	"time"

	RGSW "github.com/CDSL-EncryptedControl/CDSL/utils/core/RGSW"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rgsw"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
)

// 반복 파라미터
const (
	// addr     = "127.0.0.1:9000"
	addr     = "192.168.0.115:8080"
	numIters = 500
	period   = 0 * time.Millisecond
)

func ms(d time.Duration) float64 { return float64(d) / 1e6 }

func main() {
	// ======== Parameters (저장 당시와 동일) ========
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN: 12, LogQ: []int{56}, LogP: []int{51}, NTTFlag: true,
	})
	ringQ := params.RingQ()

	// Controller dims & quant (저장 당시와 동일)
	n, m, p := 4, 1, 2
	s := 1 / 10000.0
	L := 1 / 1000.0
	r := 1 / 10000.0

	// tau & monomials (EncPack/Unpack 셋업)
	maxDim := math.Max(math.Max(float64(n), float64(m)), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))
	logn := int(math.Log2(float64(tau)))
	monomials := make([]ring.Poly, logn)
	for i := 0; i < logn; i++ {
		monomials[i] = ringQ.NewPoly()
		idx := params.N() - params.N()/(1<<(i+1))
		monomials[i].Coeffs[0][idx] = 1
		ringQ.MForm(monomials[i], monomials[i])
		ringQ.NTT(monomials[i], monomials[i])
	}

	// ======== Load artifacts ========
	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw")

	recoveredX := new(rlwe.Ciphertext)
	if err := com_utils.ReadRT(filepath.Join(base, "xCtPack.dat"), recoveredX); err != nil {
		log.Fatalf("load xCtPack: %v", err)
	}
	ctF, err := com_utils.LoadRGSWPack(base, "ctF")
	if err != nil {
		log.Fatal(err)
	}
	ctG, err := com_utils.LoadRGSWPack(base, "ctG")
	if err != nil {
		log.Fatal(err)
	}
	ctH, err := com_utils.LoadRGSWPack(base, "ctH")
	if err != nil {
		log.Fatal(err)
	}
	ctJ, err := com_utils.LoadRGSWPack(base, "ctJ")
	if err != nil {
		log.Fatal(err)
	}

	rlk := new(rlwe.RelinearizationKey)
	if err := com_utils.ReadRT(filepath.Join(base, "rlk.dat"), rlk); err != nil {
		log.Fatal(err)
	}
	gks, err := com_utils.LoadGaloisKeys(base)
	if err != nil {
		log.Fatal(err)
	}

	evkRGSW := rlwe.NewMemEvaluationKeySet(rlk)
	evkRLWE := rlwe.NewMemEvaluationKeySet(rlk, gks...)
	evaluatorRGSW := rgsw.NewEvaluator(params, evkRGSW)
	evaluatorRLWE := rlwe.NewEvaluator(params, evkRLWE)

	zeroCt := rlwe.NewCiphertext(params, 1)

	// ======== TCP server ========
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	fmt.Println("[Controller] Listening on", addr, "...")

	conn, err := ln.Accept()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	rbuf := bufio.NewReader(conn)
	wbuf := bufio.NewWriter(conn)

	// ======== Timers (accumulators) ========
	var sumRecv, sumUnpack, sumComputeU, sumSend, sumUpdate, sumIter time.Duration
	itersDone := 0

	// ======== Control loop ========
	for it := 0; it < numIters; it++ {
		iterStart := time.Now()

		// 1) receive y
		t := time.Now()
		yCtPack := new(rlwe.Ciphertext)
		if _, err := yCtPack.ReadFrom(rbuf); err != nil {
			log.Printf("[Controller] Read yCtPack err at iter %d: %v (stop)", it, err)
			break
		}
		dRecv := time.Since(t)

		// 2) unpack x,y
		t = time.Now()
		xCt := RLWE.UnpackCt(recoveredX, n, tau, evaluatorRLWE, ringQ, monomials, params)
		yCt := RLWE.UnpackCt(yCtPack, p, tau, evaluatorRLWE, ringQ, monomials, params)
		dUnpack := time.Since(t)

		// 3) compute u = Hx + Jy
		t = time.Now()
		uCtPack := RGSW.MultPack(xCt, ctH, evaluatorRGSW, ringQ, params)
		JyCt := RGSW.MultPack(yCt, ctJ, evaluatorRGSW, ringQ, params)
		uCtPack = RLWE.Add(uCtPack, JyCt, zeroCt, params)
		dComputeU := time.Since(t)

		// 4) send u
		t = time.Now()
		if _, err := uCtPack.WriteTo(wbuf); err != nil {
			log.Printf("[Controller] Write uCtPack err at iter %d: %v (stop)", it, err)
			break
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[Controller] Flush err at iter %d: %v (stop)", it, err)
			break
		}
		dSend := time.Since(t)

		// 5) update x = F*x + G*y
		t = time.Now()
		FxCt := RGSW.MultPack(xCt, ctF, evaluatorRGSW, ringQ, params)
		GyCt := RGSW.MultPack(yCt, ctG, evaluatorRGSW, ringQ, params)
		recoveredX = RLWE.Add(FxCt, GyCt, zeroCt, params)
		dUpdate := time.Since(t)

		dIter := time.Since(iterStart)

		sumRecv += dRecv
		sumUnpack += dUnpack
		sumComputeU += dComputeU
		sumSend += dSend
		sumUpdate += dUpdate
		sumIter += dIter
		itersDone++

		// per-iter 로그 (원하면 주석 해제)
		fmt.Printf(
			"[Controller] iter=%d | recv=%.3f ms, unpack=%.3f ms, computeU=%.3f ms, send=%.3f ms, update=%.3f ms, total=%.3f ms\n",
			it, ms(dRecv), ms(dUnpack), ms(dComputeU), ms(dSend), ms(dUpdate), ms(dIter),
		)

		if period > 0 {
			time.Sleep(period)
		}
	}

	if itersDone > 0 {
		fmt.Printf(
			"[Controller][AVG over %d iters] recv=%.3f ms, unpack=%.3f ms, computeU=%.3f ms, send=%.3f ms, update=%.3f ms, total=%.3f ms\n",
			itersDone,
			ms(sumRecv)/float64(itersDone),
			ms(sumUnpack)/float64(itersDone),
			ms(sumComputeU)/float64(itersDone),
			ms(sumSend)/float64(itersDone),
			ms(sumUpdate)/float64(itersDone),
			ms(sumIter)/float64(itersDone),
		)
	}

	fmt.Println("[Controller] Done. (r,s,L) =", r, s, L, "| m =", m)
}
