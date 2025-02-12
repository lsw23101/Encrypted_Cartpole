Capstone_encryted_control
=============
2025 Capstone project repository

Requirment
=============
Go 설치하기
인터넷

Usage
=============

암호화 없이 단순한 plant controller 통신
-----------------------------------------
1. Rasberry의 Communicate_data_trans.go 실행
```cd ~/Rassberry
  go run Communicate_data_trans.go
```

2. 새로운 터미널 열고 Computer의 Communicate_data_receive.go 실행
'
  cd ~/Computer
  go run Communicate_data_receive.go
'


암호화 된 데이터 보내고 받기 
-----------------------------------------
1. Rasberry의 Enc_data_trans.go 실행
'
   cd ~/Rassberry
   go run Enc_data_trans.go
'

3. 새로운 터미널 열고 Computer의 Enc_data_receive.go 실행
'
  cd ~/Computer
  go run Enc_data_receive.go
'


