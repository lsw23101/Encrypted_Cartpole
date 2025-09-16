// === 아두이노 코드 ===
// 모터 제어 핀
const int MOTOR_DIR_PIN = 12;
const int MOTOR_PWM_PIN = 11;

// 엔코더 핀
const int encoderPinA = 2;
const int encoderPinB = 7;
volatile long encoderCount = 0;

// ADC 센서
float ADCvalue = 0;
float currentAngle = 0;
const float ADCmin = 104.0;
const float ADCmax = 919.0;
const float ANGLE_OFFSET = 77.3 -1.73;

// 타겟값
double targetAngle = 0.0;
double targetPosition = 0.0;


// 출력 벡터 y
double y[2] = {0, 0};

// 제어 입력 u (라즈베리에서 받음)
double u = 0.0;

unsigned long lastControlTime = 0;
const unsigned long controlInterval = 50; // 50ms
bool isRunning = true;



void setup() {
  Serial.begin(115200);

  pinMode(MOTOR_DIR_PIN, OUTPUT);
  pinMode(MOTOR_PWM_PIN, OUTPUT);

  pinMode(encoderPinA, INPUT_PULLUP);
  pinMode(encoderPinB, INPUT_PULLUP);
  attachInterrupt(digitalPinToInterrupt(encoderPinA), updateEncoder, CHANGE);

  Serial.println("Arduino ready.");
}




void loop() {
  if (isRunning && millis() - lastControlTime >= controlInterval) {
    lastControlTime += controlInterval;

    // 1. ADC → 각도
    ADCvalue = analogRead(A0);
    currentAngle = (ADCvalue - ADCmin) * 360.0 / (ADCmax - ADCmin);
    currentAngle = constrain(currentAngle, 0.0, 360.0);

    // 오프셋 보정
    currentAngle += ANGLE_OFFSET;
    if (currentAngle < 0) currentAngle += 360.0;
    if (currentAngle >= 360.0) currentAngle -= 360.0;

    // -180 ~ 180 변환
    float relativeAngle = currentAngle;
    if (relativeAngle > 180.0) relativeAngle -= 360.0;

    // 2. 오차 계산
    double angleError = targetAngle - relativeAngle;
    double positionError = targetPosition - getCartDistanceM();

    y[0] = angleError;
    y[1] = positionError;

    // 3. y값 전송
    Serial.print(y[0], 6);
    Serial.print(",");
    Serial.println(y[1], 6);
  }
  // 4. 라즈베리에서 u 수신
  if (Serial.available() > 0) {
    String data = Serial.readStringUntil('\n');
    u = data.toDouble();
    
    // print recieved control input 
    Serial.print(",");
    Serial.println(u, 6);
    // 5. 모터 구동
    int pwmValue = constrain(abs(u), 0, 255);
    moveMotor(pwmValue, -u > 0); // negetive feedback 
  }
}


float getCartDistanceM() {
  return (encoderCount * 2 * PI) / 255.0; // 엔코더 255 CPR 기준
}

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

    
