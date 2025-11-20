const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 3;

float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 25.0;

unsigned long lastCommandTime = 0;
const unsigned long timeoutDuration = 200;

String commandBuffer = "";

float readFilteredADC(int pin, int sampleCount = 10) {
  long total = 0;
  for (int i = 0; i < sampleCount; i++) {
    total += analogRead(pin);
    delayMicroseconds(5);
  }
  return total / (float)sampleCount;
}

void setup() {
  Serial.begin(115200);
  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);
}

void loop() {
  // 각도 계산
  ADCvalue = readFilteredADC(A0, 10);
  currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
  currentAngle = constrain(currentAngle, 0.0, 360.0);

  currentAngle += ANGLE_OFFSET;
  if (currentAngle < 0) currentAngle += 360.0;
  if (currentAngle >= 360.0) currentAngle -= 360.0;

  float relativeAngle = currentAngle;
  if (relativeAngle > 180.0) relativeAngle -= 360.0;

  // **문자열로 angle 전송 (char 기반)**
  Serial.println(String(relativeAngle, 4));

  // 시리얼 명령 수신
  while (Serial.available()) {
    char c = Serial.read();
    if (c == '\n') {
      handleCommand(commandBuffer);
      commandBuffer = "";
    } else if (c != '\r') {
      commandBuffer += c;
    }
  }

  // 타임아웃 시 모터 정지
  if (millis() - lastCommandTime > timeoutDuration) {
    stopMotor();
  }

  delay(20); // 통신 안정용
}

void handleCommand(String command) {
  int sepIndex = command.indexOf(',');
  if (sepIndex != -1) {
    int pwm = command.substring(0, sepIndex).toInt();
    int direction = command.substring(sepIndex + 1).toInt();
    moveMotor(pwm, direction == 1);
    lastCommandTime = millis();
  }
}

void moveMotor(int pwm, bool forward) {
  pwm = constrain(pwm, 0, 130);
  analogWrite(MOTOR_PWM_PIN, pwm);
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
}

void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}
