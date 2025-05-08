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
	//listener, err := net.Listen("tcp", "192.168.0.5:8080") // 내부 통신으로 변경
	listener, err := net.Listen("tcp", "127.0.0.1:8080") // 내부 통신으로 변경
	if err != nil {
		fmt.Println("TCP 리스너 실패:", err)
		return
	}
	defer listener.Close()
	fmt.Println("PID 서버 실행 중")

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("연결 수락 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("클라이언트 연결됨")

	// PID 게인 설정
	//Kp := 20
	// Ki := 0.0
	// Kd := 40.0
	// setpoint := 0.0
	// var integral, prevError float64

	// reader := bufio.NewReader(conn)

	// log2 of polynomial degree
	logN := 12
	// Choose the size of plaintext modulus (2^ptSize)
	ptSize := uint64(28)
	// Choose the size of ciphertext modulus (2^ctSize)
	ctSize := int(74)

	// ============== Encryption settings ==============
	// Search a proper prime to set plaintext modulus
	primeGen := ring.NewNTTFriendlyPrimesGenerator(ptSize, uint64(math.Pow(2, float64(logN)+1)))
	ptModulus, _ := primeGen.NextAlternatingPrime()
	fmt.Println("Plaintext modulus:", ptModulus)

	// Create a chain of ciphertext modulus
	logQ := []int{int(math.Floor(float64(ctSize) * 0.5)), int(math.Ceil(float64(ctSize) * 0.5))}

	// Parameters satisfying 128-bit security
	// BGV scheme is used
	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
		LogN:             logN,
		LogQ:             logQ,
		PlaintextModulus: ptModulus,
	})

	//eval := bgv.NewEvaluator(params, nil)

	for {
		start := time.Now() // 루프 시작 시간

		// 플랜트 출력을 담을 cipher message
		Ycin := rlwe.NewCiphertext(params, params.MaxLevel())

		// 여기서 131406은 파라미터로 설정한 q값에 따른 ct의 바이너리 크기
		totalData, err := com_utils.ReadFullData(conn, 131406)
		if err != nil {
			fmt.Println("Ycin 데이터 수신 실패:", err)
			return
		}
		err = Ycin.UnmarshalBinary(totalData[:131406])
		if err != nil {
			// 오류 로그 출력
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			return
		}

		//fmt.Println("Ycin :", Ycin)

		// Uout, _ := eval.MulNew(Ycin, Kp)
		Uout := Ycin

		//fmt.Println("Uout :", Uout)

		serialized_Uout, err := Uout.MarshalBinary() // 이런 식으로

		// 컨트롤러의 제어 입력 계산 값은 바이너리 사이즈가 131406보다 커짐 196966
		fmt.Println("Uout 크기.", len(serialized_Uout))

		_, err = conn.Write([]byte(serialized_Uout))
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}

		elapsed := time.Since(start)
		fmt.Printf("루프 처리 시간: %v\n", elapsed)
	}
}
