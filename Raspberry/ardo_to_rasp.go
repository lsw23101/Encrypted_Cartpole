package main

import (
	"bufio"
	"fmt"
	"go.bug.st/serial"
	"strconv"
	"strings"
	"time"
)

// ===== PID 계수 =====
const (
	Kp = 34.0
	Ki = 2.0
	Kd = 42.0

	Lp = 40.0
	Li = 0.0
	Ld = 3.0
)

// ===== 상태공간 행렬 =====
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

// ===== 제어기 행렬 =====
var C = []float64{Ki, -Kd, Li, -Ld}
var D = []float64{Kp + Ki + Kd, Lp + Li + Ld}

// ===== 상태, 출력, 입력 =====
var state = []float64{0, 0, 0, 0}
var y = []float64{0, 0}
var u = 0.0

// ===== 시간 유틸 =====
func ms(d time.Duration) float64 { return float64(d) / 1e6 }

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

	var iter int
	var lastLoopEnd time.Time

	for {
		loopStart := time.Now()

		// --- 1) 시리얼 수신 ---
		t := time.Now()
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		dRecv := time.Since(t)

		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ",") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}

		// --- 2) 파싱 ---
		t = time.Now()
		y0, err0 := strconv.ParseFloat(parts[0], 64)
		y1, err1 := strconv.ParseFloat(parts[1], 64)
		if err0 != nil || err1 != nil {
			continue
		}
		y[0] = y0
		y[1] = y1
		dParse := time.Since(t)

		// --- 3) 제어 계산 (u = Cx + Dy) ---
		t = time.Now()
		u = C[0]*state[0] + C[1]*state[1] + C[2]*state[2] + C[3]*state[3] +
			D[0]*y[0] + D[1]*y[1]
		dCompute := time.Since(t)

		// --- 4) 상태 업데이트 ---
		t = time.Now()
		state[0] += y[0]
		state[1] = y[0]
		state[2] += y[1]
		state[3] = y[1]
		dUpdate := time.Since(t)

		// --- 5) 송신 ---
		t = time.Now()
		_, err = port.Write([]byte(fmt.Sprintf("%.6f\n", u)))
		if err != nil {
			fmt.Println("송신 실패:", err)
			continue
		}
		dSend := time.Since(t)

		loopEnd := time.Now()
		totalLoop := loopEnd.Sub(loopStart)

		// --- 루프 간 간격 출력 ---
		if !lastLoopEnd.IsZero() {
			interval := loopStart.Sub(lastLoopEnd)
			fmt.Printf("Loop interval: %.3f ms\n", ms(interval))
		}
		lastLoopEnd = loopEnd

		// --- 결과 및 타임스탬프 출력 ---
		fmt.Printf(
			"[Loop %d] y=[%.3f, %.3f], u=%.3f | recv=%.3fms | parse=%.3fms | compute=%.3fms | update=%.3fms | send=%.3fms | total=%.3fms\n",
			iter, y0, y1, u,
			ms(dRecv), ms(dParse), ms(dCompute), ms(dUpdate), ms(dSend), ms(totalLoop),
		)

		iter++
	}
}
