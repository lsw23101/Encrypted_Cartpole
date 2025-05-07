package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"go.bug.st/serial"
)

func main() {
	// 시리얼 포트 설정 
	// 아두이노 <-> 라즈베리파이
	mode := &serial.Mode{
		BaudRate: 9600,
	}
	port, err := serial.Open("/dev/ttyACM0", mode)
	if err != nil {
		fmt.Println("시리얼 포트 열기 실패:", err)
		return
	}
	defer port.Close()
	fmt.Println("시리얼 포트 연결됨")


	// PC와 통신
	// TCP 클라이언트 연결 (자기 자신과 통신)
	conn, err := net.Dial("tcp", "192.168.0.5:8080")
	// conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		fmt.Println("TCP 연결 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("PID 서버와 TCP 연결됨")

	serialReader := bufio.NewReader(port)
	tcpReader := bufio.NewReader(conn)

	for {
		// 아두이노로 값 받기
		line, err := serialReader.ReadString('\n')
		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ",") {
			continue
		}

		fmt.Println("받은 데이터 (Angle, Distance):", line)

		// 서버로 데이터 전송
		_, err = conn.Write([]byte(line + "\n"))
		if err != nil {
			fmt.Println("서버로 데이터 전송 실패:", err)
			break
		}

		// Q: 타입 슬립 필요할까 ? 

		// 서버에서 제어값 수신
		response, err := tcpReader.ReadString('\n')
		if err != nil {
			fmt.Println("서버 응답 수신 실패:", err)
			break
		}
		response = strings.TrimSpace(response)

		// 제어값 시리얼로 전송
		_, err = port.Write([]byte(response + "\n"))
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}

		fmt.Println("PWM/Dir 전송:", response)
		time.Sleep(10 * time.Millisecond)
	}
}
