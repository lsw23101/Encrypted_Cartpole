package com_utils

import (
	"bufio"
	"fmt"
	"os"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
)

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
