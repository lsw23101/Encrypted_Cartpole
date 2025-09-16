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
	mode := &serial.Mode{
		BaudRate: 115200,
	}
	port, err := serial.Open("COM4", mode)
	if err != nil {
		fmt.Println("아두이노와 연결 실패:", err)
		return
	}
	defer port.Close()
	fmt.Println("아두이노와 연결됨")

	//conn, err := net.Dial("tcp", "192.168.0.5:8080") 지우면 안됨 !!
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

		// ① 시리얼에서 읽기 (여러 줄 올 수 있으니 마지막 줄만 처리)
		line, err := serialReader.ReadString('\n')
		if err != nil {
			fmt.Println("시리얼 읽기 실패:", err)
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 여러 줄이 들어왔다면 마지막 줄만 사용
		lines := strings.Split(line, "\n")
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		if lastLine == "" {
			continue
		}

		// 콤마가 있으면 아두이노 각도값이 아니므로 스킵
		if strings.Contains(lastLine, ",") {
			continue
		}

		fmt.Println("① 시리얼 수신 완료 (마지막 줄):", lastLine)

		// ② TCP로 각도값 전송
		_, err = conn.Write([]byte(lastLine + "\n"))
		if err != nil {
			fmt.Println("서버로 데이터 전송 실패:", err)
			break
		}
		fmt.Println("② TCP 전송 완료")

		// ③ TCP 서버에서 PWM, 방향 응답 수신 (예: "100,1")
		response, err := tcpReader.ReadString('\n')
		if err != nil {
			fmt.Println("서버 응답 수신 실패:", err)
			break
		}
		response = strings.TrimSpace(response)
		fmt.Println("③ TCP 응답 수신 완료:", response)

		// ④ 아두이노로 PWM, 방향 명령 전송
		_, err = port.Write([]byte(response + "\n"))
		if err != nil {
			fmt.Println("아두이노로 전송 실패:", err)
			break
		}
		fmt.Println("④ 시리얼 송신 완료:", response)

		// 얘가 루프 시간 설정

		time.Sleep(1 * time.Millisecond)

		loopElapsed := time.Since(loopStart)
		fmt.Printf("총 루프 처리 시간: %v\n", loopElapsed)
		fmt.Println("-----------------------------------")
	}
}
