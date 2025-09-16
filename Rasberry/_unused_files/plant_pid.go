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
	mode := &serial.Mode{
		BaudRate: 115200,
	}
	//port, err := serial.Open("/dev/ttyACM0", mode) // 아두이노와 라즈베리파이 연결 포트
	port, err := serial.Open("COM4", mode) // 컴퓨터랑 연결할때 COM4에 연결
	if err != nil {
		fmt.Println("아두이노와 실패:", err)
		return
	}
	defer port.Close()
	fmt.Println("아두이노와 연결됨")

	// TCP 클라이언트 연결
	//conn, err := net.Dial("tcp", "192.168.0.5:8080")
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		fmt.Println("TCP 연결 실패:", err)
		return
	}
	defer conn.Close()
	fmt.Println("컨트롤러 서버와 TCP 연결됨")

	serialReader := bufio.NewReader(port)
	tcpReader := bufio.NewReader(conn)

	for {
		loopStart := time.Now()

		// ① 시리얼 읽기
		t1 := time.Now()
		line, err := serialReader.ReadString('\n')
		t2 := time.Now()
		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, ",") {
			continue
		}
		fmt.Printf("① 시리얼 수신 완료 (%v)\n", t2.Sub(t1))
		fmt.Println("받은 데이터 (Angle, Distance):", line)

		// ② TCP 송신
		t3 := time.Now()
		_, err = conn.Write([]byte(line + "\n"))
		t4 := time.Now()
		if err != nil {
			fmt.Println("서버로 데이터 전송 실패:", err)
			break
		}
		fmt.Printf("② TCP 전송 완료 (%v)\n", t4.Sub(t3))

		// ③ TCP 응답 수신
		t5 := time.Now()
		response, err := tcpReader.ReadString('\n')
		t6 := time.Now()
		if err != nil {
			fmt.Println("서버 응답 수신 실패:", err)
			break
		}
		response = strings.TrimSpace(response)
		fmt.Printf("③ TCP 응답 수신 완료 (%v)\n", t6.Sub(t5))

		// ④ 시리얼 송신
		t7 := time.Now()
		_, err = port.Write([]byte(response + "\n"))
		t8 := time.Now()
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}
		fmt.Printf("④ 시리얼 송신 완료 (%v)\n", t8.Sub(t7))
		fmt.Println("PWM/Dir 전송:", response)

		time.Sleep(1000 * time.Millisecond)

		loopElapsed := time.Since(loopStart)
		fmt.Printf("총 루프 처리 시간: %v\n", loopElapsed)
		fmt.Println("-----------------------------------")
	}
}
