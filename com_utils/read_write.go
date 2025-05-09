package com_utils

import (
	"bufio"
	"fmt"
	"net"
	"os"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

func ReadFullData(conn net.Conn, expectedSize int) ([]byte, error) {
	totalData := make([]byte, 0, expectedSize)
	buf := make([]byte, 726) // 청크 크기 조절 확인 필요

	for len(totalData) < expectedSize {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("수신 오류: %v", err)
		}
		totalData = append(totalData, buf[:n]...)
	}
	return totalData, nil
}

func WriteToFile(data interface{}, filename string) error {
	// "enc_data" 폴더가 없으면 생성
	err := os.MkdirAll("enc_data", os.ModePerm)
	if err != nil {
		return fmt.Errorf("폴더 생성 오류: %v", err)
	}

	// 파일 경로에 "enc_data/" 폴더 추가
	filePath := fmt.Sprintf("enc_data/%s", filename)

	// 파일 생성
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("파일 생성 오류: %v", err)
	}
	defer file.Close()

	// bufio.Writer를 사용하여 버퍼링된 writer 생성
	bufferedWriter := bufio.NewWriter(file)

	// WriteTo 메소드를 사용하여 데이터를 파일에 씁니다.
	// 여기서 data가 WriteTo 메소드를 구현한 타입이어야 함
	switch v := data.(type) {
	case *rlwe.Ciphertext:
		// Ciphertext 타입일 경우
		if _, err := v.WriteTo(bufferedWriter); err != nil {
			return fmt.Errorf("암호문 쓰기 오류: %v", err)
		}
	case *rlwe.SecretKey:
		// SecretKey 타입일 경우
		if _, err := v.WriteTo(bufferedWriter); err != nil {
			return fmt.Errorf("비밀키 쓰기 오류: %v", err)
		}
	default:
		return fmt.Errorf("지원되지 않는 타입입니다: %T", v)
	}

	// 버퍼를 플러시하여 모든 데이터가 파일에 기록되도록 합니다.
	if err := bufferedWriter.Flush(); err != nil {
		return fmt.Errorf("버퍼 플러시 오류: %v", err)
	}

	return nil
}

func ReadFromFile(filename string, data interface{}) error {
	// 파일 경로에 "enc_data/" 폴더 추가
	filePath := fmt.Sprintf("enc_data/%s", filename)

	// 파일 열기
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("파일 열기 오류: %v", err)
	}
	defer file.Close()

	// bufio.Reader를 사용하여 버퍼링된 reader 생성
	bufferedReader := bufio.NewReader(file)

	// ReadFrom 메서드를 사용하여 데이터를 파일에서 읽어옵니다.
	// 여기서 data가 ReadFrom 메소드를 구현한 타입이어야 함
	switch v := data.(type) {
	case *rlwe.Ciphertext:
		// Ciphertext 타입일 경우
		if _, err := v.ReadFrom(bufferedReader); err != nil {
			return fmt.Errorf("암호문 읽기 오류: %v", err)
		}
	case *rlwe.SecretKey:
		// SecretKey 타입일 경우
		if _, err := v.ReadFrom(bufferedReader); err != nil {
			return fmt.Errorf("비밀키 읽기 오류: %v", err)
		}
	default:
		return fmt.Errorf("지원되지 않는 타입입니다: %T", v)
	}

	return nil
}

// ReadAndParseSerial reads a line from the serial reader, and returns parsed angle and distance. 작동안함
// func ReadAndParseSerial(reader *bufio.Reader) (angle float64, distance float64, err error) {
// 	line, err := reader.ReadString('\n')
// 	if err != nil {
// 		return 0, 0, err
// 	}

// 	fmt.Printf("⚠️ 원시 시리얼 입력: [%s]\n", line) // 원시 데이터 출력

// 	line = strings.TrimSpace(line)
// 	fmt.Printf("⚠️ Trimmed line: [%s]\n", line) // 트림된 데이터 출력

// 	if line == "" || !strings.Contains(line, ",") {
// 		return 0, 0, errors.New("invalid or empty serial data")
// 	}

// 	parts := strings.Split(line, ",")
// 	if len(parts) != 2 {
// 		return 0, 0, errors.New("malformed serial data")
// 	}

// 	angle, err1 := strconv.ParseFloat(parts[0], 64)
// 	distance, err2 := strconv.ParseFloat(parts[1], 64)
// 	if err1 != nil || err2 != nil {
// 		return 0, 0, errors.New("failed to parse float values")
// 	}

// 	return angle, distance, nil
// }
