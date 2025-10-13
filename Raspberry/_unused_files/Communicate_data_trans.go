/*
라즈베리파이: 플랜트에서 출력 y를 보내는 코드
컴퓨터에서는 Communicate_data_recieve 코드를 돌리면 됨

x+ = Ax + Bu
y = Cx

위와 같은 행렬 벡터 연산을 한 후에 통신을 보내는 역할
*/
package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// 플랜트 모델링
var A = [][]float64{
	{1.000000000000000, 0.473973933337938, 0.600315871226410, 0.080174457989503},
	{0, 0.877379985761510, 3.766788430048635, 0.600315871226410},
	{0, -0.102094535922859, 8.137309260957283, 1.450180424251285},
	{0, -0.640610277219155, 44.946391469278069, 8.137309260957284},
}

var B = [][]float64{
	{0.260260666620620},
	{1.226200142384902},
	{1.020945359228588},
	{6.406102772191552},
}

var C = [][]float64{
	{1, 0, 0, 0},
	{0, 0, 1, 0},
}

// 플랜트 초기 상태
var xp0 = []float64{
	0.000,
	0.000,
	0.01, // 0.0524 // 3 degree
	0.000,
}

func main() {
	// 서버 소켓 설정
	/*
		여기서 172.20.61.165 이 ip 주소는
		이 코드를 돌리는 컴퓨터의 ip
		cmd 에서 ifconfig 로 찾을 수 있음
		8080은 포트 이름
	*/

	listen, err := net.Listen("tcp", "172.20.61.165:8080") //
	if err != nil {
		fmt.Println("서버 소켓 설정 실패:", err)
		os.Exit(1)
	}

	defer listen.Close()
	fmt.Println("플랜트 서버 실행 중...")

	// 클라이언트와 연결 수락
	conn, err := listen.Accept()
	if err != nil {
		fmt.Println("연결 수락 실패:", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("컨트롤러와 연결됨:", conn.RemoteAddr())

	// 초기 상태
	x := xp0                     // 초기 상태 설정
	temp_x := make([]float64, 4) // x 연산 저장용 변수
	y := make([]float64, 2)      // 출력값 리스트

	// 초기 출력값 계산
	y[0] = C[0][0]*x[0] + C[0][1]*x[1] + C[0][2]*x[2] + C[0][3]*x[3]
	y[1] = C[1][0]*x[0] + C[1][1]*x[1] + C[1][2]*x[2] + C[1][3]*x[3]

	// 초기 출력값 전송
	initialY := fmt.Sprintf("%.15f,%.15f", y[0], y[1])
	_, err = conn.Write([]byte(initialY)) // 리스트 값을 문자열로 전송
	if err != nil {
		fmt.Println("초기 출력값 전송 실패:", err)
		return
	}
	fmt.Printf("초기 출력값 전송: [%s]\n", initialY)

	// 입력값 처리 루프
	for {
		// 입력값 수신
		buf := make([]byte, 200000000)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("입력값 수신 실패:", err)
			break
		}
		// 받은 데이터를 uData라는 곳에 저장
		uData := string(buf[:n])
		if uData == "" { // 연결 종료 시 루프 탈출
			break
		}

		// 입력값 예외 처리
		// 받은 데이터(stirng)를 u(float)로 타입에 맞게 저장
		u, err := strconv.ParseFloat(strings.TrimSpace(uData), 64)
		if err != nil {
			fmt.Println("잘못된 입력값:", uData)
			continue
		}

		// 플랜트 동역학 계산
		// Ax + Bu 라는 행렬  벡터 연
		for i := 0; i < 4; i++ {
			temp_x[i] = 0
			for j := 0; j < 4; j++ {
				temp_x[i] += A[i][j] * x[j]
			}
			temp_x[i] += B[i][0] * u
		}

		copy(x, temp_x) // 상태 갱신

		// 출력값 계산
		y[0] = C[0][0]*x[0] + C[0][1]*x[1] + C[0][2]*x[2] + C[0][3]*x[3]
		y[1] = C[1][0]*x[0] + C[1][1]*x[1] + C[1][2]*x[2] + C[1][3]*x[3]

		// 출력값 전송
		outputY := fmt.Sprintf("%.15f,%.15f", y[0], y[1])
		_, err = conn.Write([]byte(outputY)) // 리스트 값을 문자열로 전송
		if err != nil {
			fmt.Println("출력값 전송 실패:", err)
			break
		}

		// 출력값 로그
		fmt.Printf("입력값: %.15f, 출력값: [%s]\n", u, outputY)
	}
}
