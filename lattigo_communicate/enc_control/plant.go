// package main

// // 데이터 받고 암호화 해서 보내기

// import (
// 	"fmt"
// 	"lattigo_communicate/com_utils"
// 	"math"
// 	"net"
// 	"os"

// 	"github.com/CDSL-EncryptedControl/CDSL/utils"
// 	"github.com/tuneinsight/lattigo/v6/core/rlwe"
// 	"github.com/tuneinsight/lattigo/v6/ring"
// 	"github.com/tuneinsight/lattigo/v6/schemes/bgv"
// )

// func main() {
// 	// *****************************************************************
// 	// ************************* User's choice *************************
// 	// *****************************************************************
// 	// ============== Encryption parameters ==============
// 	// Refer to ``Homomorphic encryption standard''

// 	// log2 of polynomial degree
// 	logN := 12
// 	// Choose the size of plaintext modulus (2^ptSize)
// 	ptSize := uint64(28)
// 	// Choose the size of ciphertext modulus (2^ctSize)
// 	ctSize := int(74)

// 	// ============== Plant model ==============
// 	A := [][]float64{
// 		{0.998406460921939, 0, 0.00417376927758289, 0},
// 		{0, 0.998893625478993, 0, -0.00332671872292611},
// 		{0, 0, 0.995822899329324, 0},
// 		{0, 0, 0, 0.996671438596397},
// 	}
// 	B := [][]float64{
// 		{0.00831836513049678, 9.99686131895421e-06},
// 		{-5.19664522845810e-06, 0.00627777465144397},
// 		{0, 0.00477571210746992},
// 		{0.00311667643652227, 0},
// 	}
// 	C := [][]float64{
// 		{0.500000000000000, 0, 0, 0},
// 		{0, 0.500000000000000, 0, 0},
// 	}
// 	// Plant initial state
// 	xp0 := []float64{
// 		1,
// 		1,
// 		1,
// 		1,
// 	}

// 	// transpose of Yini from conversion.m
// 	yy0 := [][]float64{
// 		{-168.915339084001, 152.553129120773},
// 		{0, 0},
// 		{0, 0},
// 		{37.1009230518511, -33.8787596718866},
// 	}
// 	// transpose of Uini from conversion.m
// 	uu0 := [][]float64{
// 		{0, 0},
// 		{151.077820919228, -70.2395320362580},
// 		{90.8566491021641, -42.4186053244263},
// 		{54.6591007720606, -25.4768092703056},
// 	}

// 	// ============== Quantization parameters ==============
// 	r := 0.00020
// 	s := 0.00010
// 	fmt.Println("Scaling parameters 1/r:", 1/r, "1/s:", 1/s)
// 	// *****************************************************************
// 	// *****************************************************************

// 	// ============== Encryption settings ==============
// 	// Search a proper prime to set plaintext modulus
// 	primeGen := ring.NewNTTFriendlyPrimesGenerator(ptSize, uint64(math.Pow(2, float64(logN)+1)))
// 	ptModulus, _ := primeGen.NextAlternatingPrime()
// 	fmt.Println("Plaintext modulus:", ptModulus)

// 	// Create a chain of ciphertext modulus
// 	logQ := []int{int(math.Floor(float64(ctSize) * 0.5)), int(math.Ceil(float64(ctSize) * 0.5))}

// 	// Parameters satisfying 128-bit security
// 	// BGV scheme is used
// 	params, _ := bgv.NewParametersFromLiteral(bgv.ParametersLiteral{
// 		LogN:             logN,
// 		LogQ:             logQ,
// 		PlaintextModulus: ptModulus,
// 	})
// 	fmt.Println("Ciphertext modulus:", params.QBigInt())
// 	fmt.Println("Degree of polynomials:", params.N())

// 	// Generate secret key
// 	// kgen := bgv.NewKeyGenerator(params)
// 	// sk := kgen.GenSecretKeyNew()

// 	// 키 원래 만든거로 계속 가기
// 	sk := rlwe.NewSecretKey(params) // 이거는 빈 sk 만드는 함수
// 	com_utils.ReadFromFile("sk.dat", sk)

// 	encryptor := bgv.NewEncryptor(params, sk)
// 	decryptor := bgv.NewDecryptor(params, sk)
// 	encoder := bgv.NewEncoder(params)

// 	bredparams := ring.GenBRedConstant(params.PlaintextModulus())

// 	// ==============  Encryption of controller ==============
// 	// dimensions
// 	n := len(A)
// 	l := len(C)
// 	m := len(B[0])
// 	h := int(math.Max(float64(l), float64(m)))

// 	// duplicate
// 	yy0vec := make([][]float64, n)
// 	uu0vec := make([][]float64, n)
// 	for i := 0; i < n; i++ {
// 		yy0vec[i] = utils.VecDuplicate(yy0[i], m, h)
// 		uu0vec[i] = utils.VecDuplicate(uu0[i], m, h)
// 	}

// 	// Plaintext of past inputs and outputs
// 	ptY := make([]*rlwe.Plaintext, n)
// 	ptU := make([]*rlwe.Plaintext, n)

// 	// Ciphertext of past inputs and outputs
// 	ctY := make([]*rlwe.Ciphertext, n)
// 	ctU := make([]*rlwe.Ciphertext, n)

// 	// Quantization - packing - encryption
// 	for i := 0; i < n; i++ {
// 		ptY[i] = bgv.NewPlaintext(params, params.MaxLevel())
// 		encoder.Encode(utils.ModVecFloat(utils.RoundVec(utils.ScalVecMult(1/r, yy0vec[i])), params.PlaintextModulus()), ptY[i])
// 		ctY[i], _ = encryptor.EncryptNew(ptY[i])

// 		ptU[i] = bgv.NewPlaintext(params, params.MaxLevel())
// 		encoder.Encode(utils.ModVecFloat(utils.RoundVec(utils.ScalVecMult(1/r, uu0vec[i])), params.PlaintextModulus()), ptU[i])
// 		ctU[i], _ = encryptor.EncryptNew(ptU[i])

// 	}

// 	// 시뮬

// 	listen, err := net.Listen("tcp", "172.20.61.165:8080") //
// 	if err != nil {
// 		fmt.Println("서버 소켓 설정 실패:", err)
// 		os.Exit(1)
// 	}

// 	defer listen.Close()
// 	fmt.Println("플랜트 서버 실행 중...")

// 	// 클라이언트와 연결 수락
// 	conn, err := listen.Accept()
// 	if err != nil {
// 		fmt.Println("연결 수락 실패:", err)
// 		os.Exit(1)
// 	}
// 	defer conn.Close()
// 	fmt.Println("컨트롤러와 연결됨:", conn.RemoteAddr())

// 	///////////////////////////////////////////////////////////////////
// 	///////////////////////////////////////////////////////////////////
// 	// ============== Simulation ==============
// 	// Number of simulation steps
// 	iter := 10
// 	fmt.Printf("Number of iterations: %v\n", iter)

// 	// Plant state
// 	xp := xp0

// 	for i := 0; i < iter; i++ {
// 		//////////////////// **** Sensor ****

// 		// Plant output /// 1번단계!!
// 		Y := utils.MatVecMult(C, xp) // [][]float64

// 		// Quantize and duplicate // 2번 단계 !!
// 		Ysens := utils.ModVecFloat(utils.RoundVec(utils.ScalVecMult(1/r, utils.VecDuplicate(Y, m, h))), params.PlaintextModulus())
// 		Ypacked := bgv.NewPlaintext(params, params.MaxLevel())
// 		encoder.Encode(Ysens, Ypacked)
// 		Ycin, _ := encryptor.EncryptNew(Ypacked)

// 		//////////////////// 첫 번째 송신 /////////////////////////////
// 		////////////// 여기서 출력 송신
// 		// 출력값 전송
// 		fmt.Println("ct.BinarySize", Ycin.BinarySize())

// 		// 위에서 구한 Ycin 을 직렬화 해서 밑에 담기

// 		serialized_Ycin, err := Ycin.MarshalBinary() // 이런 식으로
// 		fmt.Println("lenth of serialized_Ycin: ", len(serialized_Ycin))
// 		// 패킹했으니까 하나의 값이지 않을까 예상.... 즉 통신하는건 하나의 ciphertext
// 		outputY := fmt.Sprintf("%.18f,%.18f", serialized_Ycin)
// 		_, err = conn.Write([]byte(outputY)) // 리스트 값을 문자열로 전송
// 		if err != nil {
// 			fmt.Println("출력값 전송 실패:", err)
// 			break
// 		}

// 		//////////// 여기서 u 수신 3번단계를 컨트롤러가하고
// 		///////////// 여기는 4번 단계 !!!!

// 		// 여기서 ct0는 암호공간의 메세지
// 		Uout := rlwe.NewCiphertext(params, params.MaxLevel())
// 		// 데이터 수신 버퍼 설정
// 		chunkSize := 1024

// 		buf := make([]byte, chunkSize) // 1024 바이트씩 수신
// 		// buf := make([]byte, 65000)

// 		// 데이터 수신을 위한 누적된 결과 저장
// 		var totalData []byte

// 		for {
// 			// 데이터 수신 (서버에서 전송한 바이너리 데이터 받기)
// 			n, err := conn.Read(buf)
// 			if err != nil {
// 				fmt.Println("수신 오류:", err)
// 				break
// 			}

// 			// 수신된 데이터 누적
// 			totalData = append(totalData, buf[:n]...)

// 			// 만약 전체 데이터를 다 받았으면 종료
// 			if len(totalData) >= 131406 { // 예시로 131406 크기만큼 받으면 종료
// 				break
// 			}
// 		}

// 		// 여기서 직렬화

// 		err = Uout.UnmarshalBinary(totalData[:131406])
// 		if err != nil {
// 			// 오류 로그 출력
// 			fmt.Println("Ciphertext 역직렬화 실패:", err)
// 			return
// 		}

// 		////// 여기서 4단계가 끝나고 Uout << 암호화 된 값을 받아왔음

// 		// **** Actuator **** ////////// 여기서 U cin 재암호화 한거 보내기
// 		// Plant input
// 		U := make([]float64, m)
// 		// Unpacked and re-scaled u at actuator
// 		Uact := make([]uint64, params.N())
// 		// u after inner sum
// 		Usum := make([]uint64, m)
// 		encoder.Decode(decryptor.DecryptNew(Uout), Uact)
// 		// Generate plant input
// 		for k := 0; k < m; k++ {
// 			Usum[k] = utils.VecSumUint(Uact[k*h:(k+1)*h], params.PlaintextModulus(), bredparams)
// 			U[k] = float64(r * s * utils.SignFloat(float64(Usum[k]), params.PlaintextModulus()))
// 		}

// 		// 위에 연산은 ARX관련한 그냥 그거고 결국 위에서 끝난 U가 메세지공간의 값

// 		// Re-encryption
// 		Upacked := bgv.NewPlaintext(params, params.MaxLevel())
// 		encoder.Encode(utils.ModVecFloat(utils.RoundVec(utils.ScalVecMult(1/r, utils.VecDuplicate(U, m, h))), params.PlaintextModulus()), Upacked)
// 		Ucin, _ := encryptor.EncryptNew(Upacked)

// 		// 여기서 재암호화 한 Ucin 다시 보내기 직렬화 해줘서 보내야겠지?

// 		///////////////////// 두 번째 송신 ////////////////////////////
// 		// 위에서 구한 Ucin 데이터 보내기 !!
// 		serialized_reenc_U, err := Ucin.MarshalBinary() // 이런 식으로
// 		reenc_U := fmt.Sprintf("%.15f,%.15f", serialized_reenc_U)
// 		_, err = conn.Write([]byte(reenc_U)) // 리스트 값을 문자열로 전송
// 		if err != nil {
// 			fmt.Println("출력값 전송 실패:", err)
// 			break
// 		}

// 		// **** Plant ****
// 		// State update
// 		xp = utils.VecAdd(utils.MatVecMult(A, xp), utils.MatVecMult(B, U))

// 	}

// }
