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

	utils "github.com/CDSL-EncryptedControl/CDSL/utils"
	RLWE "github.com/CDSL-EncryptedControl/CDSL/utils/core/RLWE"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

// 반복 파라미터
const (
	addr     = "127.0.0.1:9000"
	numIters = 500
	period   = 0 * time.Millisecond
)

func main() {
	// ======== Parameters (서버와 반드시 동일) ========
	params, _ := rlwe.NewParametersFromLiteral(rlwe.ParametersLiteral{
		LogN: 12, LogQ: []int{56}, LogP: []int{51}, NTTFlag: true,
	})

	// Dims & quant (서버와 동일)
	m, p := 1, 2
	s := 1 / 10000.0
	L := 1 / 1000.0
	r := 1 / 10000.0
	_ = s // (여기선 s는 복호스케일 계산에 직접 등장하진 않지만 남겨둠)

	// tau
	maxDim := math.Max(float64(m), float64(p))
	tau := int(math.Pow(2, math.Ceil(math.Log2(maxDim))))

	ringQ := params.RingQ()

	// ======== Load secret key & build cryptors ========
	base := filepath.Join("..", "Offline_task", "enc_data", "rgsw")
	sk := new(rlwe.SecretKey)
	if err := com_utils.ReadRT(filepath.Join(base, "sk.dat"), sk); err != nil {
		log.Fatalf("load sk: %v", err)
	}
	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	rbuf := bufio.NewReader(conn)
	wbuf := bufio.NewWriter(conn)
	fmt.Println("[Plant] Connected to", addr)

	for it := 0; it < numIters; it++ {
		// 1) y 생성 & 암호화
		y := []float64{0.001, 0.001} // 필요 시 시간에 따라 바꿔도 됨
		yBar := utils.RoundVec(utils.ScalVecMult(1/r, y))
		yCtPack := RLWE.EncPack(yBar, tau, 1/L, *encryptor, ringQ, params)

		// 2) 전송
		if _, err := yCtPack.WriteTo(wbuf); err != nil {
			log.Printf("[Plant] Write yCtPack err at iter %d: %v (stop)", it, err)
			break
		}
		if err := wbuf.Flush(); err != nil {
			log.Printf("[Plant] Flush err at iter %d: %v (stop)", it, err)
			break
		}
		fmt.Printf("[Plant] iter=%d sent yCtPack\n", it)

		// 3) u 수신
		uCtPack := new(rlwe.Ciphertext)
		if _, err := uCtPack.ReadFrom(rbuf); err != nil {
			log.Printf("[Plant] Read uCtPack err at iter %d: %v (stop)", it, err)
			break
		}

		// 4) 복호 & 출력
		u := RLWE.DecUnpack(uCtPack /*m=*/, 1, tau, *decryptor, r*s*s*L, ringQ, params)
		fmt.Printf("[Plant] iter=%d u(decrypted)=%v\n", it, u)

		if period > 0 {
			time.Sleep(period)
		}
	}

	fmt.Println("[Plant] Done.")
}
