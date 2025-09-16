// 모터 제어 핀
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

// 엔코더 핀
const int encoderPinA = 2;
const int encoderPinB = 7;
volatile long encoderCount = 0;

// 각도 센서
float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 110.7 - 26.1 - 3.9;

// 거리 계산
const float wheelRadiusM = 0.04;
const float wheelCircumferenceM = 2 * PI * wheelRadiusM;
const float countsPerRevolution = 268.0;

float getCartDistanceM() {
  float rotations = encoderCount / countsPerRevolution;
  return rotations * wheelCircumferenceM;
}

// PID 제어 변수 (각도)
double angleTarget = 0.0, angleError = 0.0;
double angleKp = 23.0, angleKi = 0.75, angleKd = 25.0;
double angleIntegral = 0.0, anglePrevError = 0.0;
double angleControl = 0.0;

// PID 제어 변수 (위치)
double posTarget = 0.0, posError = 0.0;
double posKp = 1.5, posKi = 0.0, posKd = 1.5;
double posIntegral = 0.0, posPrevError = 0.0;
double posControl = 0.0;

// 실행 타이머
unsigned long lastControlTime = 0;
const unsigned long controlInterval = 50;  // 내부 루프: 50ms (20Hz)

unsigned long lastPositionUpdateTime = 0;
const unsigned long positionUpdateInterval = 150;  // 외부 루프: 150ms (6.7Hz)

bool isRunning = true;

// 필터 함수
float readFilteredADC(int pin, int sampleCount = 100) {
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
  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);
  Serial.println("Press 'r' to toggle the control loop. 'f'/'b' to move forward/backward.");
}

void loop() {
  if (Serial.available() > 0) {
    char cmd = Serial.read();
    if (cmd == 'r' || cmd == 'R') {
      isRunning = !isRunning;
      if (!isRunning) stopMotor();
      Serial.println(isRunning ? "RUNNING" : "STOPPED");
    }
    else if (cmd == 'f') {
      posTarget += 0.05;  // 5cm 전진
    }
    else if (cmd == 'b') {
      posTarget -= 0.05;  // 5cm 후진
    }
  }

  if (isRunning && millis() - lastControlTime >= controlInterval) {
    lastControlTime += controlInterval;

    // 각도 측정
    ADCvalue = readFilteredADC(A0, 50);
    currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin) + ANGLE_OFFSET;
    currentAngle = fmod(currentAngle + 360.0, 360.0);
    if (currentAngle > 180.0) currentAngle -= 360.0;
    float angle = currentAngle * PI / 180.0;

    // 위치 측정
    float position = getCartDistanceM();

    // 외부 루프 (150ms마다 위치 PID 실행 → 각도 목표 갱신)
    if (millis() - lastPositionUpdateTime >= positionUpdateInterval) {
      lastPositionUpdateTime += positionUpdateInterval;

      posError = posTarget - position;
      posIntegral += posError;
      double posDeriv = posError - posPrevError;
      posPrevError = posError;

      posControl = posKp * posError + posKi * posIntegral + posKd * posDeriv;

      angleTarget = constrain(posControl, -0.3, 0.3);  // 라디안
    }

    // 내부 루프 (각도 제어)
    angleError = angleTarget - angle;
    angleIntegral += angleError;
    angleIntegral = constrain(angleIntegral, -120, 120);
    double angleDeriv = angleError - anglePrevError;
    anglePrevError = angleError;

    angleControl = angleKp * angleError + angleKi * angleIntegral + angleKd * angleDeriv;

    // 모터 제어
    double controlSignal = angleControl;
    int pwmValue = constrain(abs(controlSignal), 0, 255);
    moveMotor(pwmValue, controlSignal > 0);

    // 디버깅 출력
    static unsigned long lastDebug = 0;
    if (millis() - lastDebug > 200) {
      lastDebug = millis();
      Serial.print("Angle(deg): "); Serial.print(currentAngle, 2);
      Serial.print(" | Pos(m): "); Serial.print(position, 3);
      Serial.print(" | TargetPos(m): "); Serial.print(posTarget, 3);
      Serial.print(" | AngleTgt(rad): "); Serial.print(angleTarget, 3);
      Serial.print(" | PWM: "); Serial.println(pwmValue);
    }
  }
}

// 인터럽트 핸들러
void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) encoderCount++;
  else encoderCount--;
}

void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}
