// file: enc_plant.go
package main

import (
	"Encrypted_Cartpole/com_utils"
	"bufio"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"path/filepath"
	"time"

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// ===== 사용자 환경에 맞게 조정 =====
const (
	addr       = "192.168.0.115:8080" // 컨트롤러 주소
	// addr     = "127.0.0.1:9000"

	// RLWE params (컨트롤러와 동일해야 함)
	logN = 12
	logQ = 56
	logP = 51

	// 차원
	m = 1 // control input dimension
	p = 2 // measurement dimension

	// 양자화 스케일 (컨트롤러와 동일)
	s = 1.0 / 1.0
	L = 1.0 / 100000.0
	r = 1.0 / 10000.0
)

// 루프 주기 (조정 가능)
var period = 100 * time.Millisecond

func main() {
	// ===== RLWE 세팅 =====
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN:    logN,
		LogQ:    []int{logQ},
		LogP:    []int{logP},
		NTTFlag: true,
	})
	ringQ := params.RingQ()

	// tau: >= max(m,p) 2의 거듭제곱
	maxDim := math.Max(float64(m), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

	// SecretKey 로드
	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw")
	sk := new(rlwe.SecretKey)
	if err := com_utils.ReadRT(filepath.Join(base, "sk.dat"), sk); err != nil {
		log.Fatalf("load sk: %v", err)
	}
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)

	// ===== TCP 연결 =====
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatalf("tcp dial: %v", err)
	}
	defer conn.Close()
	rbuf := bufio.NewReader(conn)
	wbuf := bufio.NewWriter(conn)
	fmt.Println("[Test] Connected to controller:", addr)

	iter := 0
	for {
		tLoopStart := time.Now()

		// 1) y값 생성 (테스트용 고정)
		y := []float64{0.1, 0.1}

		// ===== 구간 1: 암호화 시간 측정 =====
		tEncStart := time.Now()
		yBar := utils.RoundVec(utils.ScalVecMult(1.0/r, y))
		yCtPack := RLWE.EncPack(yBar, tau, 1.0/L, *encryptor, ringQ, params)
		tEncEnd := time.Now()

		// ===== 구간 2: 통신 (송신+응답수신) =====
		tCommStart := time.Now()
		if _, err := yCtPack.WriteTo(wbuf); err != nil {
			log.Printf("[Test] Write yCtPack err at iter %d: %v", iter, err)
			break
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[Test] Flush err at iter %d: %v", iter, err)
			break
		}

		uCtPack := new(rlwe.Ciphertext)
		if _, err := uCtPack.ReadFrom(rbuf); err != nil {
			log.Printf("[Test] Read uCtPack err at iter %d: %v", iter, err)
			break
		}
		tCommEnd := time.Now()

		// ===== 구간 3: 복호화 =====
		tDecStart := time.Now()
		uVec := RLWE.DecUnpack(uCtPack /*m=*/, m, tau, *decryptor, r*s*s*L, ringQ, params)
		tDecEnd := time.Now()

		u := 0.0
		if len(uVec) > 0 {
			u = uVec[0]
		}

		// ===== 각 구간 시간 계산 =====
		T_enc := tEncEnd.Sub(tEncStart)
		T_comm := tCommEnd.Sub(tCommStart)
		T_dec := tDecEnd.Sub(tDecStart)
		RTT_total := tDecEnd.Sub(tLoopStart)

		// ===== 출력 =====
		fmt.Printf("[Test] iter=%d | y=[%.4f %.4f] -> u=%.6f | "+
			"T_enc=%v | T_comm=%v | T_dec=%v | RTT_total=%v\n",
			iter, y[0], y[1], u, T_enc, T_comm, T_dec, RTT_total)

		iter++
		time.Sleep(period)
	}

	fmt.Println("[Test] Stopped.")
}

// parseTwoFloats 함수는 여기선 사용 안 하지만, 에러 참고용으로 남겨둠.
func parseTwoFloats(line string) (float64, float64, error) {
	if line == "" {
		return 0, 0, errors.New("empty line")
	}
	return 0, 0, errors.New("unused in this test")
}
