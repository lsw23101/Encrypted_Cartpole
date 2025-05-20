import serial
import time
import sys
import select
import tty
import termios

# 시리얼 포트 설정
ser = serial.Serial('/dev/ttyACM0', 115200, timeout=0.1)
time.sleep(2)

# PID 파라미터
Kp = 20.0
Ki = 0.0
Kd = 40.0
target = 0.0
prev_error = 0.0
integral = 0.0

# 터미널 설정 백업
fd = sys.stdin.fileno()
old_settings = termios.tcgetattr(fd)
tty.setcbreak(fd)

print("Press 'r' to reset encoder. Ctrl+C to quit.\n")

try:
    while True:
        if ser.in_waiting:
            # 버퍼 내 모든 데이터 읽기
            raw_data = ser.read(ser.in_waiting).decode(errors='ignore')
            lines = raw_data.strip().split('\n')
            if lines:
                last_line = lines[-1].strip()
                if last_line:
                    parts = last_line.split(',')
                    if len(parts) == 2:
                        try:
                            angle = float(parts[0])
                            distance = float(parts[1])

                            error = angle - target
                            integral += error
                            derivative = error - prev_error
                            prev_error = error

                            output = (Kp * error) + (Ki * integral) + (Kd * derivative)
                            pwm = int(min(255, abs(output)))
                            direction = 1 if output > 0 else 0

                            command = f"{pwm},{direction}\n"
                            ser.write(command.encode())

                            print(f"Angle: {angle:.2f}° | Distance: {distance:.2f} cm | PWM: {pwm} | Dir: {direction}")
                        except ValueError:
                            print(f"[Warning] Could not convert data to float: {last_line}")

        # r 키 입력 감지 (즉시 반응)
        if select.select([sys.stdin], [], [], 0)[0]:
            ch = sys.stdin.read(1)
            if ch.lower() == 'r':
                ser.write(b"reset\n")
                print(">>> Encoder reset requested.")

        time.sleep(0.001)

except KeyboardInterrupt:
    print("\nExiting...")

finally:
    # 터미널 설정 원복
    termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)
    ser.close()
