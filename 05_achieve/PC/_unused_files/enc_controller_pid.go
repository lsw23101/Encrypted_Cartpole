package main

import (
	"Enc_control_RLWE/com_utils"
	"fmt"
	"math"
	"net"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

func main() {
	// ====== 암호화 파라미터 초기화 ======
	logN := 12
	ptSize := uint64(28)
	ctSize := int(74)

	primeGen := ring.NewNTTFriendlyPrimesGenerator(ptSize, uint64(math.Pow(2, float64(logN)+1)))
	ptModulus, _ := primeGen.NextAlternatingPrime()
	fmt.Println("Plaintext modulus:", ptModulus)

	logQ := []int{int(math.Floor(float64(ctSize) * 0.5)), int(math.Ceil(float64(ctSize) * 0.5))}
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             logQ,
		PlaintextModulus: ptModulus,
	})

	eval := bgv.NewEvaluator(params, nil)
	Kp := 20

	// ====== TCP 서버 리스닝 ======
	listener, err := net.Listen("tcp", "192.168.0.5:8080")
	if err != nil {
		fmt.Println("TCP 리스너 실패:", err)
		return
	}
	defer listener.Close()
	fmt.Println("PID 서버 실행 중 (클라이언트 반복 접속 가능)")

	for {
		fmt.Println("클라이언트 대기 중...")
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("연결 수락 실패:", err)
			continue
		}
		fmt.Println("클라이언트 연결됨:", conn.RemoteAddr())

		Ycin := rlwe.NewCiphertext(params, params.MaxLevel())

		for {
			start := time.Now()

			totalData, err := com_utils.ReadFullData(conn, 131406)
			if err != nil {
				fmt.Println("Ycin 데이터 수신 실패. 클라이언트 연결 종료:", err)
				break // 내부 루프 탈출 -> 새 클라이언트 대기
			}

			err = Ycin.UnmarshalBinary(totalData[:131406])
			if err != nil {
				fmt.Println("Ciphertext 역직렬화 실패:", err)
				break
			}

			Uout, _ := eval.MulNew(Ycin, Kp)

			serialized_Uout, err := Uout.MarshalBinary()
			if err != nil {
				fmt.Println("직렬화 실패:", err)
				break
			}

			_, err = conn.Write(serialized_Uout)
			if err != nil {
				fmt.Println("출력값 전송 실패:", err)
				break
			}

			fmt.Printf("루프 처리 시간: %v\n", time.Since(start))
		}

		conn.Close()
		fmt.Println("클라이언트와 연결 종료됨. 다음 연결 대기 중...\n")
	}
}
