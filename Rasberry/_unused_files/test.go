package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.bug.st/serial"
)

func main() {
	port, err := serial.Open("COM4", &serial.Mode{BaudRate: 115200})
	if err != nil {
		fmt.Println("포트 연결 실패:", err)
		return
	}
	defer port.Close()
	fmt.Println("포트 연결됨. 아두이노 리셋 대기 중...")
	time.Sleep(3 * time.Second)

	reader := bufio.NewReader(port)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("읽기 오류:", err)
			continue
		}

		line = strings.TrimSpace(line) // 불필요한 공백 제거
		fmt.Println(">>", line)

		// 각도, 거리 데이터 파싱
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			fmt.Println("파싱 오류: 예상 형식 'angle,distance'가 아닙니다.")
			continue
		}

		// 각도 값 추출
		angleStr := parts[0]
		fmt.Println("Angle 데이터:", angleStr)

		// 각도 파싱
		angle, err := strconv.ParseFloat(angleStr, 64)
		if err != nil {
			fmt.Println("Angle 파싱 실패:", err)
			continue
		}

		// 양자화 및 출력
		quantizedAngle := int(angle * 100) // 양자화 예시 (예: 1/0.01)
		fmt.Printf("양자화된 각도: %d\n", quantizedAngle)
	}
}
