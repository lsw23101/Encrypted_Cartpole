package main

import (
	"bufio"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
)

func main() {
	listener, err := net.Listen("tcp", "192.168.0.5") // 내부 통신으로 변경
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
	Kp := 20.0
	Ki := 0.0
	Kd := 40.0
	setpoint := 0.0
	var integral, prevError float64

	reader := bufio.NewReader(conn)

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("데이터 수신 실패:", err)
			break
		}
		line = strings.TrimSpace(line)
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			continue
		}

		angle, _ := strconv.ParseFloat(parts[0], 64)

		// PID 연산
		error := angle - setpoint
		integral += error
		derivative := error - prevError
		prevError = error

		output := Kp*error + Ki*integral + Kd*derivative
		pwm := int(math.Min(255, math.Abs(output)))
		dir := 0
		if output > 0 {
			dir = 1
		}

		response := fmt.Sprintf("%d,%d", pwm, dir)
		_, err = conn.Write([]byte(response + "\n"))
		if err != nil {
			fmt.Println("응답 전송 실패:", err)
			break
		}

		fmt.Printf("받은 각도: %.2f -> PWM: %d | Dir: %d\n", angle, pwm, dir)
	}
}
