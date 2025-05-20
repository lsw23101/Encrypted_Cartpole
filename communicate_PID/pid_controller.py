import serial
import time

def open_serial():
    while True:
        try:
            ser = serial.Serial('COM4', 115200, timeout=1)
            time.sleep(2)
            ser.reset_input_buffer()
            return ser
        except serial.SerialException:
            print("[Warning] Serial port not available, retrying in 2 seconds...")
            time.sleep(2)

def main():
    ser = open_serial()

    Kp, Ki, Kd = 400.0, 0.0, 40.0
    target = 0.0
    prev_error = 0.0
    integral = 0.0

    print("Starting PID control loop. Press Ctrl+C to exit.\n")

    try:
        while True:
            try:
                if ser.in_waiting:
                    raw_data = ser.read(ser.in_waiting).decode(errors='ignore')
                    lines = raw_data.strip().split('\n')
                    if not lines:
                        continue

                    last_line = lines[-1].strip()
                    if not last_line:
                        continue

                    angle = float(last_line)
                    error = angle - target
                    integral += error
                    derivative = error - prev_error
                    prev_error = error

                    output = (Kp * error) + (Ki * integral) + (Kd * derivative)
                    pwm = min(130, max(0, int(abs(output))))
                    direction = 1 if output > 0 else 0

                    command = f"{pwm},{direction}\n"
                    ser.write(command.encode())

                    print(f"Angle: {angle:.4f}° | PWM: {pwm} | Dir: {direction}")

                time.sleep(0.01)

            except (serial.SerialException, OSError) as e:
                print(f"[Error] Serial exception: {e}")
                try:
                    ser.close()
                except:
                    pass
                print("[Info] Attempting to reopen serial port...")
                ser = open_serial()

    except KeyboardInterrupt:
        print("\n[Info] Exiting...")

    finally:
        try:
            ser.write(b"0,0\n")
            ser.close()
        except:
            pass
        print("[Info] Serial port closed.")

if __name__ == "__main__":
    main()
