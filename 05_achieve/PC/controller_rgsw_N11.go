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

const (
	// addr = "192.168.0.20:8080" // 서버 바인딩 주소
	addr = "192.168.0.115:8080" // 서버 바인딩 주소
	// addr     = "127.0.0.1:9000" // 컨트롤러 주소
	numIters = 500 // 0 means infinite loop
	period   = 0 * time.Millisecond
)

func ms(d time.Duration) float64 { return float64(d) / 1e6 }

func main() {
	// ======== Parameters (저장 당시와 동일) ========
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN: 11, LogQ: []int{28}, LogP: []int{28}, NTTFlag: true,
	})
	ringQ := params.RingQ()

	// Controller dims & quant (저장 당시와 동일)
	n, m, p := 4, 1, 2
	s := 1 / 5.0
	L := 1 / 300.0
	r := 1 / 50.0
	_ = s
	_ = L
	_ = r

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
	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw_for_N11")

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

	// ======== Accumulators (avg만 출력) ========
	var sumRecv, sumUnpack, sumComputeU, sumSend, sumUpdate, sumIter time.Duration
	var sumRecvBytes, sumSentBytes int64
	var sumSendToNextRecv time.Duration // 이전 루프 send 완료 → 다음 루프 y 수신 완료까지
	var commSamples int                 // 유효 표본 수 (최초 루프 제외)
	itersDone := 0

	// 이전 루프의 send 완료 시각
	var lastSendDone time.Time
	var haveLastSend bool

	// for it := 0; it < numIters; it=++ {
	for it := 0; ; it = it { // 무한루프
		iterStart := time.Now()

		// 1) receive y (ReadFrom 1회)
		t := time.Now()
		yCtPack := new(rlwe.Ciphertext)
		nRecv, err := yCtPack.ReadFrom(rbuf)
		if err != nil {
			log.Printf("[Controller] Read yCtPack err at iter %d: %v (stop)", it, err)
			break
		}
		dRecv := time.Since(t)
		sumRecvBytes += nRecv

		// ★ 통신시간(컨트롤러 관점): 직전 루프에서 u를 다 보낸 시각 → 이번 루프에서 y를 다 받은 시각
		if haveLastSend {
			sumSendToNextRecv += time.Since(lastSendDone)
			commSamples++
		}

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

		// 4) send u (WriteTo 1회)
		t = time.Now()
		nSent, err := uCtPack.WriteTo(wbuf)
		if err != nil {
			log.Printf("[Controller] Write uCtPack err at iter %d: %v (stop)", it, err)
			break
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[Controller] Flush err at iter %d: %v (stop)", it, err)
			break
		}
		dSend := time.Since(t)
		sumSentBytes += nSent

		// ★ 다음 루프 통신시간 계산을 위해, 이번 루프의 send 완료 시각 저장
		lastSendDone = time.Now()
		haveLastSend = true

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

		if period > 0 {
			time.Sleep(period)
		}
	}

	// ======== 평균만 출력 ========
	if itersDone > 0 {
		avgRecv := ms(sumRecv) / float64(itersDone)
		avgUnpack := ms(sumUnpack) / float64(itersDone)
		avgComputeU := ms(sumComputeU) / float64(itersDone)
		avgSend := ms(sumSend) / float64(itersDone)
		avgUpdate := ms(sumUpdate) / float64(itersDone)
		avgTotal := ms(sumIter) / float64(itersDone)

		avgYKB := float64(sumRecvBytes) / float64(itersDone) / 1024.0
		avgUKB := float64(sumSentBytes) / float64(itersDone) / 1024.0

		var avgSendToNextRecv float64
		if commSamples > 0 {
			avgSendToNextRecv = ms(sumSendToNextRecv) / float64(commSamples)
		}

		fmt.Printf("[Controller][AVG over %d] recv=%.3f ms, unpack=%.3f ms, computeU=%.3f ms, send=%.3f ms, update=%.3f ms, total=%.3f ms | bytes avg y=%.1f KB, avg u=%.1f KB\n",
			itersDone, avgRecv, avgUnpack, avgComputeU, avgSend, avgUpdate, avgTotal, avgYKB, avgUKB)

		// 컨트롤러 관점의 통신시간(이전 send 완료 → 다음 y 수신 완료)
		if commSamples > 0 {
			fmt.Printf("[Controller][AVG comm over %d samples] send->nextRecv = %.3f ms\n", commSamples, avgSendToNextRecv)
		}
	}

	fmt.Println("[Controller] Done. (r,s,L) =", r, s, L, "| m =", m)
}
