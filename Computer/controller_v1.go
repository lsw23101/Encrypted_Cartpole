package main

import (
	"fmt"
	"lattigo_communicate/com_utils"
	"math"
	"net"
	"time"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

func readFullData(conn net.Conn, expectedSize int) ([]byte, error) {
	totalData := make([]byte, 0, expectedSize)
	buf := make([]byte, 726) // 청크 크기를 작게 설정 (4KB)

	for len(totalData) < expectedSize {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("수신 오류: %v", err)
		}
		totalData = append(totalData, buf[:n]...)
	}
	return totalData, nil
}

func main() {
	// *****************************************************************
	// ************************* User's choice *************************
	// *****************************************************************
	// ============== Encryption parameters ==============
	// Refer to ``Homomorphic encryption standard''

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

	// 컨트롤러는 evaluator만 가지고 있어야 함

	eval := bgv.NewEvaluator(params, nil)

	// ==============  Encryption of controller ==============
	// dimensions
	n := 4

	// 여기서 ctHy랑 ctHu 파일 읽기

	// 암호문 읽고 복원하기 + u랑 y 시퀀스 초기값도 읽어와야됨됨
	recovered_ctHu := make([]*rlwe.Ciphertext, 4) // 크기 4로 초기화
	recovered_ctHy := make([]*rlwe.Ciphertext, 4) // 크기 4로 초기화
	// Ciphertext of past inputs and outputs
	recovered_ctY := make([]*rlwe.Ciphertext, n)
	recovered_ctU := make([]*rlwe.Ciphertext, n)

	// 각 인덱스에 새로운 Ciphertext 객체 생성
	for i := 0; i < 4; i++ {
		recovered_ctHu[i] = rlwe.NewCiphertext(params, 1)
		recovered_ctHy[i] = rlwe.NewCiphertext(params, 1)
		recovered_ctU[i] = rlwe.NewCiphertext(params, 1)
		recovered_ctY[i] = rlwe.NewCiphertext(params, 1)
	}

	// 복원
	//recovered_sk := rlwe.NewSecretKey(params) // 빈 sk 만드는 함수
	for i := 0; i < 4; i++ {
		filename_Hu := fmt.Sprintf("ctHu[%d].dat", i)
		if err := com_utils.ReadFromFile(filename_Hu, recovered_ctHu[i]); err != nil {
			fmt.Println("파일 읽기 실패:", filename_Hu, err)
			return
		}

		filename_Hy := fmt.Sprintf("ctHy[%d].dat", i)
		if err := com_utils.ReadFromFile(filename_Hy, recovered_ctHy[i]); err != nil {
			fmt.Println("파일 읽기 실패:", filename_Hy, err)
			return
		}

		filename_u := fmt.Sprintf("ctU[%d].dat", i)
		if err := com_utils.ReadFromFile(filename_u, recovered_ctU[i]); err != nil {
			fmt.Println("파일 읽기 실패:", filename_u, err)
			return
		}

		filename_y := fmt.Sprintf("ctY[%d].dat", i)
		if err := com_utils.ReadFromFile(filename_y, recovered_ctY[i]); err != nil {
			fmt.Println("파일 읽기 실패:", filename_y, err)
			return
		}
	}

	// fmt.Println("recovered_ctHy[0]:", recovered_ctHy[0])
	// fmt.Println("recovered_ctY[0]:", recovered_ctY[0])
	// fmt.Println("recovered_ctHu[0]:", recovered_ctHu[0])
	// fmt.Println("recovered_ctU[0]:", recovered_ctU[0])

	// 이건 읽으면 안됨
	// com_utils.ReadFromFile("sk.dat", recovered_sk)

	// 컨트롤러 소켓 설정
	// 소켓 연결

	// conn, err := net.Dial("tcp", "127.0.0.1:8080") // 서버에서 설정한 ip와 동일한 ip, 즉 라즈베리 파이의 ip
	conn, err := net.Dial("tcp", "192.168.0.50:8080") // 서버에서 설정한 ip와 동일한 ip, 즉 라즈베리 파이의 ip
	if err != nil {
		fmt.Println("서버에 연결 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("컨트롤러와 연결됨:", conn.RemoteAddr())

	///////////////////////////////////////////////////////////////////
	///////////////////////////////////////////////////////////////////
	// ============== Simulation ==============
	// Number of simulation steps
	iter := 500
	fmt.Printf("Number of iterations: %v\n", iter)

	// 2) Plant + encrypted controller

	for i := 0; i < iter; i++ {
		fmt.Println(i+1, "번째 이터레이션")
		loop_start := time.Now()

		// 여기서 Cin 받고 역직렬화 하기 // 여기가 플랜트의 2번단계와 연동
		// 출력값 수신 (서버에서 y값 받기)

		// 여기서 Ycin는 암호공간의 메세지
		Ycin := rlwe.NewCiphertext(params, params.MaxLevel())
		// 데이터 수신 버퍼 설정
		// chunkSize := 726

		// buf := make([]byte, chunkSize) // 1024 바이트씩 수신
		// buf := make([]byte, 65000)

		// 데이터 수신을 위한 누적된 결과 저장
		var totalData []byte

		fmt.Println("첫번째 통신 시작 지점 ")

		totalData, err := readFullData(conn, 131406)
		if err != nil {
			fmt.Println("Ycin 데이터 수신 실패:", err)
			return
		}

		// 마샬링 시간 계산
		fmt.Println("Ycin 받는데 걸리는 시간:", time.Since(loop_start))

		// 여기서 직렬화
		fmt.Println("Ycin 길이 ", len(totalData))
		err = Ycin.UnmarshalBinary(totalData[:131406])
		if err != nil {
			// 오류 로그 출력
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			return
		}

		fmt.Println("Ycin 직렬화 걸리는 시간:", time.Since(loop_start))

		// 지금의 Ycin << 이건 6번 단계에서 쓰일 예정

		///
		//// 여기가 3번 단계
		// **** Encrypted controller ****

		Uout, _ := eval.MulNew(recovered_ctHy[0], recovered_ctY[0])
		// fmt.Println("이쯤에서 디버그", Uout)

		eval.MulThenAdd(recovered_ctHu[0], recovered_ctU[0], Uout)
		for j := 1; j < n; j++ {
			eval.MulThenAdd(recovered_ctHy[j], recovered_ctY[j], Uout)
			eval.MulThenAdd(recovered_ctHu[j], recovered_ctU[j], Uout)
		}

		fmt.Println("여기서 출력 계산 끝남:", time.Since(loop_start))
		// 위에서 구한 Uout 데이터 보내기 !!

		//// 헐 !!! 여기서는 Uout이 바이너리 사이즈가 늘어났어 !!
		// fmt.Println("Uout ct.BinarySize", Uout.BinarySize())
		serialized_Uout, err := Uout.MarshalBinary() // 이런 식으로

		fmt.Println("Y 크기.", len(serialized_Uout))

		_, err = conn.Write([]byte(serialized_Uout)) // 리스트 값을 문자열로 전송
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}

		fmt.Println("여기서 U 전송 끝남: ", time.Since(loop_start))

		// 5번단계랑 대응되는 재암호화 값 받기 !!!

		// buf_reenc := make([]byte, chunkSize) // 1024 바이트씩 수신
		// buf := make([]byte, 65000)

		// 데이터 수신을 위한 누적된 결과 저장
		// var totalData_reenc []byte
		// 여기서 Ycin는 암호공간의 메세지
		Ucin := rlwe.NewCiphertext(params, params.MaxLevel())

		// 둘 사이 간격

		/// 꼭 이 타이밍에 간격을 줘야 제대로 작동

		// 2. ACK 수신
		ackBuf := make([]byte, 3)
		_, err = conn.Read(ackBuf)
		if err != nil || string(ackBuf) != "ACK" {
			fmt.Println("ACK 수신 실패:", err)
			return
		}
		fmt.Println("ACK 수신 완료, 재암호화 데이터 받기 시작")

		fmt.Println("재암호화 받기 전:", time.Since(loop_start))
		totalData_reenc, err := readFullData(conn, 131406)
		if err != nil {
			fmt.Println("Ycin 데이터 수신 실패:", err)
			return
		}
		fmt.Println("재암호화 받은 후:", time.Since(loop_start))
		// 여기서 직렬화

		err = Ucin.UnmarshalBinary(totalData_reenc[:131406])
		if err != nil {
			// 오류 로그 출력
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			return
		}
		// **** Encrypted Controller **** 6번 단계 !!!!!!
		// State update
		recovered_ctY = append(recovered_ctY[1:], Ycin)
		recovered_ctU = append(recovered_ctU[1:], Ucin)

		fmt.Println("한 루프 걸리는 시간:", time.Since(loop_start))

		// 2. ACK 수신
		conn.Write([]byte("ACK"))
		fmt.Println("ACK 전송 완료")
	}

}
