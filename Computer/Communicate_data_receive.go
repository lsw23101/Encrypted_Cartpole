package main

import (
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ============== Pre-designed controller ==============
var F = [][]float64{
	{0.040988052143164, 0.521313168922281, -2.698002752828384, -0.287227644618238},
	{-0.537578869168603, 1.100415522278173, -13.263178342743721, -1.130673741532966},
	{-0.381155576071777, 0.083606874539275, -15.916248344351882, 0.008942773942124},
	{-2.072179204361123, 0.524606166156636, -94.582736823785297, -0.905992051473843},
}

var G = [][]float64{
	{0.967635515356980, 1.262486295588120},
	{0.578208211411685, 7.438283467544731},
	{0.414983934899813, 16.067435096273435},
	{2.284441242978658, 89.418785871946042},
}

var H = [][]float64{
	{0.033134348006240, 0.181891625035102, -7.822282002505928, -1.411669720892939},
}

var xc0 = []float64{
	0,
	0,
	0,
	0,
}

var u0 = 0.000

func main() {
	// 컨트롤러 소켓 설정
	conn, err := net.Dial("tcp", "172.20.61.165:8080") // 서버에서 설정한 ip와 동일한 ip, 즉 라즈베리 파이의 ip
	if err != nil {
		fmt.Println("서버에 연결 실패:", err)
		return
	}
	defer conn.Close()

	// CSV 파일 생성
	file, err := os.Create("controller_data.csv")
	if err != nil {
		fmt.Println("CSV 파일 생성 실패:", err)
		return
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// CSV 헤더 작성
	writer.Write([]string{"Iteration", "xc1", "xc2", "xc3", "xc4", "u", "ElapsedTime(ms)"})

	// 컨트롤러에서 상태 벡터 xc의 초기값
	xc := xc0
	temp_x := make([]float64, 4) // x 연산 저장용 변수 설정
	uComputed := u0

	// 이터레이션 속도 추적 변수
	var totalTime int64

	// 입력값 처리 루프 (100회 반복)
	for iteration := 1; iteration <= 100; iteration++ {
		// 이터레이션 시작 시간 기록
		startTime := time.Now()

		// 출력값 수신 (서버에서 y값 받기)
		buf := make([]byte, 200000000)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("출력값 수신 실패:", err)
			break
		}
		yData := string(buf[:n])

		// 출력값 y를 배열로 변환
		yStrings := strings.Split(strings.TrimSpace(yData), ",")
		if len(yStrings) != 2 {
			fmt.Println("출력값 배열 크기 불일치:", yData)
			continue
		}

		// y 배열 초기화 및 변환
		y := make([]float64, 2)
		for i, s := range yStrings {
			y[i], err = strconv.ParseFloat(strings.TrimSpace(s), 64)
			if err != nil {
				fmt.Println("출력값 변환 실패:", err)
				break
			}
		}

		// 계산된 u 값 전송
		_, err = conn.Write([]byte(fmt.Sprintf("%.23f", uComputed)))
		if err != nil {
			fmt.Println("입력값 전송 실패:", err)
			break
		}

		// 상태 벡터 xc를 업데이트 (플랜트 동역학 계산)
		for i := 0; i < 4; i++ {
			temp_x[i] = F[i][0]*xc[0] + F[i][1]*xc[1] + F[i][2]*xc[2] + F[i][3]*xc[3]
			for j := 0; j < 2; j++ {
				temp_x[i] += G[i][j] * y[j]
			}
		}
		copy(xc, temp_x) // temp_x -> xc 업데이트

		// 출력값 계산 (제어 입력 u 계산)
		uComputed = 0
		for i := 0; i < 4; i++ {
			uComputed += H[0][i] * xc[i]
		}

		// 이터레이션 종료 시간 기록 및 경과 시간 계산 (밀리초로 변경)
		elapsedTime := time.Since(startTime).Milliseconds() // 밀리초로 변경
		totalTime += elapsedTime

		// 현재 데이터 CSV에 기록
		row := []string{
			strconv.Itoa(iteration),
			fmt.Sprintf("%.15f", xc[0]),
			fmt.Sprintf("%.15f", xc[1]),
			fmt.Sprintf("%.15f", xc[2]),
			fmt.Sprintf("%.15f", xc[3]),
			fmt.Sprintf("%.15f", uComputed),
			strconv.FormatInt(elapsedTime, 10),
		}
		writer.Write(row)

		// 출력값 로그
		fmt.Printf("Iteration %d: xc = %.15f, %.15f, %.15f, %.15f | u = %.15f | Time = %d ms\n",
			iteration, xc[0], xc[1], xc[2], xc[3], uComputed, elapsedTime)

		// 0.05초 딜레이 추가 (플랜트의 응답 속도에 맞추기 위해)
		time.Sleep(50 * time.Millisecond)
	}

	// 평균 시간 계산 및 출력
	averageTime := totalTime / 100
	fmt.Printf("Average time per iteration: %d ms\n", averageTime)

	fmt.Println("CSV 데이터 저장 완료.")
}
