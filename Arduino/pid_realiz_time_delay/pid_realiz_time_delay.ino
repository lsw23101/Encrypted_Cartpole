// 모터 제어 핀 (MD10C 방식)
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

// 인터럽트 핀 (A상, B상 엔코더용)
const int encoderPinA = 2;
const int encoderPinB = 7;

// 엔코더 관련 변수
volatile long encoderCount = 0;

// 아날로그 각도 센서 변수
float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 77.3 -1.73;

double targetAngle = 0.0;
double targetPosition = 0.0;
double angleError = 0.0;
double positionError = 0.0;

const double Kp = 34, Ki = 4, Kd = 42;
const double Lp = 40, Li = 0, Ld = 3;

const double A[4][4] = {{1, 0, 0, 0},
                        {0, 0, 0, 0},
                        {0, 0, 1, 0},
                        {0, 0, 0, 0}};

const double B[4][2] = {{1, 0},
                        {1, 0},
                        {0, 1},
                        {0, 1}};

const double C[1][4] = {{Ki, -Kd, Li, -Ld}};

const double D[1][2] = {{Kp+Ki+Kd, Lp+Li+Ld}};

double state[4][1] = {{0},
                      {0},
                      {0},
                      {0}};

double y[2][1] = {{0},
                  {0}};

double u[1][1] = {{0}};



// PID 합산 
double controlSignal = 0.0;

// 타이머
unsigned long lastControlTime = 0;
// 제어 샘플링 타임 50ms
const unsigned long controlInterval = 50;
// 사용안함
// const long samplingTime = controlInterval / 1000;

// 실행 상태
bool isRunning = true;

// 바퀴 정보 사용 x
const float wheelRadiusM = 0.04;
const float wheelCircumferenceM = 2 * PI * wheelRadiusM;
const float countsPerRevolution = 255.0;

// 거리 계산 함수(라디안 단위)
float getCartDistanceM() {
  return (encoderCount *2*PI) / countsPerRevolution;
  //return (encoderCount);
}

float cartPosition = 0;

// ADC 평균 필터 함수
float readFilteredADC(int pin, int sampleCount = 100) {
  long total = 0;
  for (int i = 0; i < sampleCount; i++) {
    total += analogRead(pin);
    delayMicroseconds(5);
  }
  return total / (float)sampleCount;
}

// 시리얼 입력 처리용 버퍼
char serialBuffer[20];
byte bufferIndex = 0;

void setup() {
  Serial.begin(115200);

  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);

  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  Serial.println("Press 'r' to start/stop the control loop.");
  Serial.println("Use 'p:value' to set angle Kp, 'pp:value' for position Kp");
}

void loop() 
{
  // 시리얼 입력 처리
  while (Serial.available() > 0) 
  {
    char input = Serial.read();
    // 실행 상태 토글
    if (input == 'r' || input == 'R') 
    {
      isRunning = !isRunning;
      if (!isRunning) 
      {
        stopMotor();
        Serial.println("System STOPPED.");
        state[0][0] = 0;
        state[1][0] = 0;
        state[2][0] = 0;
        state[3][0] = 0;
        y[0][0] = 0;
        y[1][0] = 0;
        u[0][0] = 0;
        encoderCount = 0;

      } 
      else 
      {
        Serial.println("System RUNNING.");
      }
    }
  }

  // ----------------------------------------------------- //

  // 제어 루프
  if (isRunning && millis() - lastControlTime >= controlInterval) 
  {
    lastControlTime += controlInterval;

  // ----------------------------------------------------- //
    // 1. ADC → 각도
    ADCvalue = readFilteredADC(A0, 100);
    currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
    currentAngle = constrain(currentAngle, 0.0, 360.0);

    // 2. 오프셋 보정
    currentAngle += ANGLE_OFFSET;
    if (currentAngle < 0) currentAngle += 360.0;
    if (currentAngle >= 360.0) currentAngle -= 360.0;

    // 3. -180 ~ +180 변환
    float relativeAngle = currentAngle;
    if (relativeAngle > 180.0) relativeAngle -= 360.0;

    // 4. 각도 오차
    targetAngle = 0.0;
    // angleError = relativeAngle - targetAngle;
    angleError = targetAngle - relativeAngle;

    // 5. 위치 오차
    cartPosition = getCartDistanceM();
    positionError = targetPosition - cartPosition;

    // 여기까지 과정에서 y=[theta, alpha] 진자각도 바퀴각 target이 0 이니까
    y[0][0] = angleError;
    y[1][0] = positionError;
    
  // ----------------------------------------------------- //

    // 제어 입력 인가
    u[0][0] = C[0][0] * state[0][0] + C[0][1] * state[1][0] + C[0][2] * state[2][0] 
            + C[0][3] * state[3][0] + D[0][0] * y[0][0] + D[0][1] * y[1][0];

    int pwmValue = constrain(abs(-u[0][0]), 0, 255);
    moveMotor(pwmValue, -u[0][0] > 0);

    Serial.print("y[0][0] = ");
    Serial.print(y[0][0]);
    Serial.print(", y[1][0] = ");
    Serial.print(y[1][0]);
    Serial.print(" | u[0][0] = ");
    Serial.println(-u[0][0]);

//    Serial.print("state[0][0] = ");
//    Serial.print(state[0][0]);
//    Serial.print(", state[1][0] = ");
//    Serial.print(state[1][0]);
//    Serial.print(", state[2][0] = ");
//    Serial.print(state[2][0]);
//    Serial.print(", state[3][0] = ");
//    Serial.println(state[3][0]);

    // 상태 업데이트 
    
    state[0][0] = state[0][0] + y[0][0];
    state[1][0] =               y[0][0];
    state[2][0] = state[2][0] + y[1][0];
    state[3][0] =               y[1][0];

//    if(state[0][0] > 120 || state[0][0] < -120)
//    {
//      state[0][0] = state[0][0] > 0 ? 120 : -120;
//    }
//    if(state[2][0] > 120 || state[2][0] < -120)
//    {
//      state[2][0] = state[2][0] > 0 ? 120 : -120;
//    }
  }
}

// 엔코더 인터럽트
void updateEncoder() {
  bool A = digitalRead(encoderPinA);
  bool B = digitalRead(encoderPinB);
  if (A == B) {
    encoderCount++;
  } else {
    encoderCount--;
  }
}

// 모터 제어
void moveMotor(int pwm, bool forward) {
  digitalWrite(MOTOR_DIR_PIN, forward ? HIGH : LOW);
  analogWrite(MOTOR_PWM_PIN, pwm);
}

// 모터 정지
void stopMotor() {
  analogWrite(MOTOR_PWM_PIN, 0);
}
