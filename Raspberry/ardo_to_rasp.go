package main

import (
	"bufio"
	"fmt"
	"go.bug.st/serial"
	"strconv"
	"strings"
	"time"
)

var lastTime time.Time

// PID 계수
const (
	Kp = 34.0
	Ki = 2.0
	Kd = 42.0

	Lp = 40.0
	Li = 0.0
	Ld = 3.0
)

// 상태공간 행렬 (여기서는 직접 사용하지 않지만 원래 정의 반영)
var A = [4][4]float64{
	{1, 0, 0, 0},
	{0, 0, 0, 0},
	{0, 0, 1, 0},
	{0, 0, 0, 0},
}

var B = [4][2]float64{
	{1, 0},
	{1, 0},
	{0, 1},
	{0, 1},
}

// 제어기 출력 행렬
var C = []float64{Ki, -Kd, Li, -Ld}
var D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

// 상태, 출력, 입력
var state = []float64{0, 0, 0, 0} // 4x1
var y = []float64{0, 0}           // 2x1
var u = 0.0                       // 1x1



func main() {
	mode := &serial.Mode{BaudRate: 115200}
	port, err := serial.Open("/dev/ttyACM0", mode) // 아두이노 연결
	if err != nil {
		fmt.Println("시리얼 연결 실패:", err)
		return
	}
	defer port.Close()

	reader := bufio.NewReader(port)

	fmt.Println("라즈베리파이 제어기 시작")

	for {
		line, err := reader.ReadString('\n')

		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ",") {
			continue
		}




		// 1. y값 파싱
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}


		y0, err0 := strconv.ParseFloat(parts[0], 64)
		y1, err1 := strconv.ParseFloat(parts[1], 64)
		if err0 != nil || err1 != nil {
		    continue
		}

		now := time.Now()
		if !lastTime.IsZero() {
			interval := now.Sub(lastTime)
			fmt.Printf("Loop interval: %v\n", interval)
		}
		lastTime = now

		y[0] = y0
		y[1] = y1

		// 2. 제어 입력 u 계산 (u = Cx + Dy)
		u = C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]


		// 3. 상태 업데이트 (간단한 적분기/미분기 모델)
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]


		
		fmt.Printf("받은 y = [%.3f, %.3f], 보낸 u = %.3f\n", y0, y1, u)


		// 4. 아두이노로 송신
		port.Write([]byte(fmt.Sprintf("%.6f\n", u)))

		
		
	}
}





