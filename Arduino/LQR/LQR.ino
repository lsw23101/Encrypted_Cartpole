// 모터 제어 핀
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

// 엔코더 핀
const int encoderPinA = 2;
const int encoderPinB = 7;

// 엔코더 변수
volatile long encoderCount = 0;

// 각도 센서 변수
float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 110.7 - 26.1;

// 타이머
unsigned long lastControlTime = 0;
const unsigned long controlInterval = 50;  // 50ms

bool isRunning = true;

// 바퀴 거리 계산
const float wheelRadiusCM = 4.0;
const float wheelCircumferenceCM = 2 * PI * wheelRadiusCM;
const int encoderPPR = 38;
const int gearRatio = 14;
const int countsPerRevolution = encoderPPR * gearRatio * 4;
const float distanceCorrectionFactor = 10.0 / 9.21;

float getCartDistanceM() {
  float rotations = encoderCount / (float)countsPerRevolution;
  return (rotations * wheelCircumferenceCM * distanceCorrectionFactor) / 100.0;
}

// 상태 변수
float x_hat[4] = {0.0, 0.0, 0.0, 0.0};

// 상태공간 행렬 (업데이트된 값)
float F_mat[4][4] = {
  {0.11126129, -0.03104599, -0.13371149, -0.02763648},
  {4.43719034, -2.23797570, -5.10791536, -1.10420334},
  {-0.17904715, 0.16026396, 0.06712872, 0.09495324},
  {-6.30730891, 5.79238321, 2.81572220, 2.59402296}
};

float G_mat[4][2] = {
  {0.99980471, 0.01144085},
  {9.2043e-05, 0.22295014},
  {3.5049e-05, 1.15454758},
  {0.00072079, 5.11537604}
};

float H_vec[4] = {
  34.53131138, -25.13837663, -37.94316040, -8.59120869
};

// ADC 필터
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
  Serial.println("Press 'r' to start/stop.");
}

void loop() {
  if (Serial.available() > 0) {
    char input = Serial.read();
    if (input == 'r' || input == 'R') {
      isRunning = !isRunning;
      if (!isRunning) {
        stopMotor();
        Serial.println("System STOPPED.");
      } else {
        Serial.println("System RUNNING.");
      }
    }
  }

  if (isRunning && millis() - lastControlTime >= controlInterval) {
    lastControlTime += controlInterval;

    // 1. ADC → 각도
    ADCvalue = readFilteredADC(A0, 50);
    currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
    currentAngle += ANGLE_OFFSET;

    float relativeAngle = currentAngle;
    while (relativeAngle > 180.0) relativeAngle -= 360.0;
    while (relativeAngle < -180.0) relativeAngle += 360.0;

    float angleRad = relativeAngle * PI / 180.0;
    float cartPosition = getCartDistanceM();  // m 단위

    // 2. 측정값 → 상태 추정 초기화
    x_hat[0] = cartPosition;
    x_hat[2] = angleRad;

    // 3. 상태 추정 갱신: x⁺ = Fx + Gy
    float x_hat_next[4] = {0.0, 0.0, 0.0, 0.0};
    for (int i = 0; i < 4; i++) {
      for (int j = 0; j < 4; j++) {
        x_hat_next[i] += F_mat[i][j] * x_hat[j];
      }
      x_hat_next[i] += G_mat[i][0] * cartPosition + G_mat[i][1] * angleRad;
    }
    for (int i = 0; i < 4; i++) {
      x_hat[i] = x_hat_next[i];
    }

    // 4. 제어 입력 계산: u = Hx
    float u = 0;
    for (int i = 0; i < 4; i++) {
      u += H_vec[i] * x_hat[i];
    }

    // 5. PWM 출력 변환
    int pwm = constrain((int)(abs(u) * 255.0 / 12.0), 0, 255);
    bool forward = u > 0;
    moveMotor(pwm, forward);

    // 6. 디버깅 출력
    static unsigned long lastDebugTime = 0;
    if (millis() - lastDebugTime >= 200) {
      lastDebugTime = millis();
      Serial.print("[Input] Pos(m): "); Serial.print(cartPosition, 3);
      Serial.print(" | Angle(rad): "); Serial.print(angleRad, 3);
      Serial.print(" || [x̂] ");
      for (int i = 0; i < 4; i++) {
        Serial.print("x"); Serial.print(i + 1); Serial.print(": ");
        Serial.print(x_hat[i], 3);
        Serial.print(i < 3 ? ", " : "");
      }
      Serial.print(" || [Control] U(V): "); Serial.print(u, 3);
      Serial.print(" | PWM: "); Serial.print(pwm);
      Serial.print(" | Dir: "); Serial.println(forward ? "FWD" : "REV");
    }
  }
}

void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) {
    encoderCount++;
  } else {
    encoderCount--;
  }
}

void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}
