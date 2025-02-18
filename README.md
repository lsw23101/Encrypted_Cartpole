Capstone_encryted_control
=============
2025 Capstone project repository

현재 30ms 이내로 한 루프

Requirment
=============
Go 설치하기

인터넷 연결 > ifconfig 로 ip 확인
or
127.0.0.1 로 설정해서 한 컴퓨터로 통신 시뮬레이션 돌려보기

Preliminary
===
1. RLWE
2. ARX form controller

Usage
=============

<terminal 1>
```
cd ~/Rasberry
```

```
go run plant.go
```

<terminal 2>
```
cd ~/Computer
```

```
go run controller.go
```

Todo
====

첫 이터레이션에서는 추가 시간 소요 <- 더미 이터레이션을 두는 것을 시도할 예정
