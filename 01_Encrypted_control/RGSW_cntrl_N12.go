package main

import (
	com_utils "Encrypted_Cartpole/03_Utils"
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
	// addr     = "192.168.20.133:8080" // 서버 바인딩 주소
	addr     = ":8080" // 서버 바인딩 주소
	numIters = 1500    // 0 means infinite loop
	period   = 0 * time.Millisecond

	printEvery = 100 // ★ 매 100회마다 요약 출력
)

func ms(d time.Duration) float64 { return float64(d) / 1e6 }

// 첫 다항식의 첫 계수를 16진수 문자열로 반환
func ctFirstCoeffHex(ct *rlwe.Ciphertext) string {
	if ct == nil || len(ct.Value) == 0 {
		return "nil"
	}
	p := ct.Value[0] // ring.Poly (값 타입)
	if len(p.Coeffs) == 0 || len(p.Coeffs[0]) == 0 {
		return "empty"
	}
	return fmt.Sprintf("0x%X", p.Coeffs[0][0])
}

func main() {
	// ======== Parameters (저장 당시와 동일) ========
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN: 12, LogQ: []int{56}, LogP: []int{51}, NTTFlag: true,
	})
	ringQ := params.RingQ()

	// Controller dims & quant (저장 당시와 동일)
	n, m, pDim := 4, 1, 2
	s := 1 / 10.0
	L := 1 / 10000.0
	r := 1 / 1000.0
	_ = s
	_ = L
	_ = r

	// tau & monomials (EncPack/Unpack 셋업)
	maxDim := math.Max(math.Max(float64(n), float64(m)), float64(pDim))
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
	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw_for_N12")

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

	// ======== 누적치 (창 누적: 매 100회마다 출력) ========
	var winRecv, winUnpack, winComputeU, winSend, winUpdate, winIter time.Duration
	var winRecvBytes, winSentBytes int64
	var winSendToNextRecv time.Duration // 이전 루프 send 완료 → 다음 루프 y 수신 완료까지
	var winCommSamples int              // 통신 표본 수
	winCount := 0

	itersDone := 0

	// 이전 루프의 send 완료 시각
	var lastSendDone time.Time
	var haveLastSend bool

	// 마지막 y/u 암호문 보관(샘플 계수 프린트용)
	var lastYct *rlwe.Ciphertext
	var lastUct *rlwe.Ciphertext

	printWindow := func() {
		if winCount == 0 {
			return
		}
		avgRecv := ms(winRecv) / float64(winCount)
		avgUnpack := ms(winUnpack) / float64(winCount)
		avgComputeU := ms(winComputeU) / float64(winCount)
		avgSend := ms(winSend) / float64(winCount)
		avgUpdate := ms(winUpdate) / float64(winCount)
		avgTotal := ms(winIter) / float64(winCount)
		avgPhase := avgUnpack + avgComputeU + avgSend + avgUpdate

		avgKB := float64(winSentBytes) / float64(winCount) / 1024.0
		if avgKB == 0 {
			avgKB = float64(winRecvBytes) / float64(winCount) / 1024.0
		}

		fmt.Printf("[Controller][AVG last %d] recv=%.3f ms, phase(unpack+computeU+send+update)=%.3f ms, total=%.3f ms | avg ct bytes ≈ %.1f KB\n",
			winCount, avgRecv, avgPhase, avgTotal, avgKB)

		if winCommSamples > 0 {
			avgComm := ms(winSendToNextRecv) / float64(winCommSamples)
			fmt.Printf("[Controller][COMM last %d] send->nextRecv = %.3f ms\n", winCommSamples, avgComm)
		}

		fmt.Printf("[Controller][CT sample coeff] yCt[0][0]=%s | uCt[0][0]=%s\n",
			ctFirstCoeffHex(lastYct), ctFirstCoeffHex(lastUct))
	}

	resetWindow := func() {
		winRecv, winUnpack, winComputeU, winSend, winUpdate, winIter = 0, 0, 0, 0, 0, 0
		winRecvBytes, winSentBytes = 0, 0
		winSendToNextRecv = 0
		winCommSamples = 0
		winCount = 0
	}

	// 메인 루프
	for {
		iterStart := time.Now()

		// 1) receive y (ReadFrom 1회)
		t := time.Now()
		yCtPack := new(rlwe.Ciphertext)
		nRecv, err := yCtPack.ReadFrom(rbuf)
		if err != nil {
			log.Printf("[Controller] Read yCtPack err at iter %d: %v (stop)", itersDone, err)
			break
		}
		dRecv := time.Since(t)
		winRecvBytes += nRecv

		// 직전 send 완료 → 이번 수신 완료까지
		if haveLastSend {
			winSendToNextRecv += time.Since(lastSendDone)
			winCommSamples++
		}

		// 2) unpack x,y
		t = time.Now()
		xCt := RLWE.UnpackCt(recoveredX, n, tau, evaluatorRLWE, ringQ, monomials, params)
		yCt := RLWE.UnpackCt(yCtPack, pDim, tau, evaluatorRLWE, ringQ, monomials, params)
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
			log.Printf("[Controller] Write uCtPack err at iter %d: %v (stop)", itersDone, err)
			break
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[Controller] Flush err at iter %d: %v (stop)", itersDone, err)
			break
		}
		dSend := time.Since(t)
		winSentBytes += nSent

		// 다음 루프 통신시간 계산용
		lastSendDone = time.Now()
		haveLastSend = true

		// 5) update x = F*x + G*y
		t = time.Now()
		FxCt := RGSW.MultPack(xCt, ctF, evaluatorRGSW, ringQ, params)
		GyCt := RGSW.MultPack(yCt, ctG, evaluatorRGSW, ringQ, params)
		recoveredX = RLWE.Add(FxCt, GyCt, zeroCt, params)
		dUpdate := time.Since(t)

		dIter := time.Since(iterStart)

		// 창 누적
		winRecv += dRecv
		winUnpack += dUnpack
		winComputeU += dComputeU
		winSend += dSend
		winUpdate += dUpdate
		winIter += dIter
		winCount++

		// 샘플 계수용
		lastYct = yCtPack
		lastUct = uCtPack

		itersDone++

		// 매 100회마다 출력 후 리셋
		if winCount >= printEvery {
			printWindow()
			resetWindow()
		}

		// 선택적 슬립
		if period > 0 {
			time.Sleep(period)
		}

		// (선택) 고정 횟수 종료
		if numIters > 0 && itersDone >= numIters {
			// 남은 창이 있으면 마지막으로 한 번 더 출력
			printWindow()
			break
		}
	}

	fmt.Println("[Controller] Done. (r,s,L) =", r, s, L, "| m =", m)
}
