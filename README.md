Capstone_encryted_control
=============
2025 Capstone project repository

Description
====
<ardu_to_rasp, rasp_to_pc, pc_to_rasp>
- 일단 PID를 realization 한 연산에 대하여 통신 구현


<plant.go and controller.go>
- Lattigo CDSL 라이브러리 두개로 나눈거 돌려보는 실행파일
- 현재 한 루프에 30ms 이내 (프린트 문을 빼도 비슷함)
- 한번씩 통신 오류 발생 (+ 한번씩 50ms 넘는 루프타임)
- com_utils : 파일 읽고 쓰기 관련 함수

- 아두이노 <<==시리얼통신==>> 라즈베리파이 <==TCP통신==>> 서버pc 환경은 완료
- 각 위치의 plant_pid.go 와 controller_pid.go 실행으로 확인 (controller 먼저 실행)
  

Requirment
=============
Go 설치하기

인터넷 연결 > ifconfig 로 ip 확인 
현재 컨트롤러용 pc 는 192.168.0.5 // 카트폴 : 192.168.0.30
or
127.0.0.1 로 설정해서 한 컴퓨터로 통신 시뮬레이션 돌려보기
(이렇게 했을 시에는 통신 오류 X)

Preliminary
===
1. RLWE
2. ARX form controller

Usage
=============



```
git clone https://github.com/lsw23101/Enc_control_RLWE
```





plant와 controller 코드에서 ip 설정

<terminal 1>
```
cd ~/Rasberry
```

// 암호 데이터 송수신 파일
```
go run plant.go 
```
// 아두이노와 통신 파일
```
go run controller_pid.go 
```



<terminal 2>
```
cd ~/Computer
```

// 암호 데이터 송수신 파일
```
go run controller.go
```
// 아두이노와 통신 파일
```
go run plant_pid.go 
```

Todo
====

통신 환경에 따라 루프 당 걸리는 시간 변동이 심함 

중간에 통신 끊기는 상황 예외 처리

코드 정리

PID 아두이노 송수신 코드에서 암호화 적용


N이 작을때의 원격 컨트롤러 구현해서 비밀 키 빼내는 부분 제작



