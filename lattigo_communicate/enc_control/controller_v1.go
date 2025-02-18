package main

import (
	"fmt"
	"lattigo_communicate/com_utils"
	"math"
	"net"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
)

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

	conn, err := net.Dial("tcp", "127.0.0.1:8080") // 서버에서 설정한 ip와 동일한 ip, 즉 라즈베리 파이의 ip
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

		// 여기서 Cin 받고 역직렬화 하기 // 여기가 플랜트의 2번단계와 연동
		// 출력값 수신 (서버에서 y값 받기)

		// 여기서 Ycin는 암호공간의 메세지
		Ycin := rlwe.NewCiphertext(params, params.MaxLevel())
		// 데이터 수신 버퍼 설정
		chunkSize := 1024

		buf := make([]byte, chunkSize) // 1024 바이트씩 수신
		// buf := make([]byte, 65000)

		// 데이터 수신을 위한 누적된 결과 저장
		var totalData []byte

		// fmt.Println("첫번째 통신 시작 지점 ")

		for {
			// 데이터 수신 (서버에서 전송한 바이너리 데이터 받기)
			n, err := conn.Read(buf)
			if err != nil {
				if err.Error() == "EOF" { // 데이터가 더 이상 없을 경우
					fmt.Println("모든 데이터 수신 완료.")
					break
				}
				fmt.Println("수신 오류:", err)
				break
			}

			// 수신된 데이터 누적
			totalData = append(totalData, buf[:n]...)

			// 만약 전체 데이터를 다 받았으면 종료
			if len(totalData) >= 131406 { // 예시로 131406 크기만큼 받으면 종료
				break
			}
		}

		// 여기서 직렬화

		err = Ycin.UnmarshalBinary(totalData[:131406])
		if err != nil {
			// 오류 로그 출력
			fmt.Println("Ciphertext 역직렬화 실패:", err)
			return
		}

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

		// 위에서 구한 Uout 데이터 보내기 !!

		//// 헐 !!! 여기서는 Uout이 바이너리 사이즈가 늘어났어 !!
		// fmt.Println("Uout ct.BinarySize", Uout.BinarySize())
		serialized_Uout, err := Uout.MarshalBinary() // 이런 식으로
		_, err = conn.Write([]byte(serialized_Uout)) // 리스트 값을 문자열로 전송
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}

		// 5번단계랑 대응되는 재암호화 값 받기 !!!

		buf_reenc := make([]byte, chunkSize) // 1024 바이트씩 수신
		// buf := make([]byte, 65000)

		// 데이터 수신을 위한 누적된 결과 저장
		var totalData_reenc []byte
		// 여기서 Ycin는 암호공간의 메세지
		Ucin := rlwe.NewCiphertext(params, params.MaxLevel())

		for {
			// 데이터 수신 (서버에서 전송한 바이너리 데이터 받기)
			n, err := conn.Read(buf_reenc)
			if err != nil {
				fmt.Println("수신 오류:", err)
				break
			}

			// 수신된 데이터 누적
			totalData_reenc = append(totalData_reenc, buf_reenc[:n]...)

			// 만약 전체 데이터를 다 받았으면 종료
			if len(totalData_reenc) >= 131406 { // 예시로 131406 크기만큼 받으면 종료
				break
			}
		}

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

	}

}
