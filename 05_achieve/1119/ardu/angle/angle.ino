#include <Wire.h>


long ADCvalue = 0; //AN25-analog의 OUT의 출력값(10bit ADC 2^10=1024; 출력값0.5V~4.5V -> 146-859 구간)
long AddADCvalue =0;  //for문에서 ADCvalue를 100번 더한 값
float angle = 0;  //long형으로 계산할 수 있는 범위를 초과해서 float형으로 정의
long minvaluex100 = 51200; //ADCvalue를 100번 더한 값인 AddADCvalue의 최소값. 초기값을 10bit ADC의 중간값512*100
long maxvaluex100 = 51200; //ADCvalue를 100번 더한 값인 AddADCvalue의 최대값. 초기값을 10bit ADC의 중간값512*100
int i=0;


void setup() {
  Serial.begin(9600);

  
}

void loop() {
  AddADCvalue=0;                        //ADCvalue를 100번 더하고 나서 초기화하기
  for (int i=0; i<100; i++)             //ADCvalue를 100번 더해서 소수점 두자리까지의 angle값을 계산해서 정확도를 높임.
  {
  ADCvalue = analogRead(A1);           
  AddADCvalue += ADCvalue;              //AddADCvalue는 ADCvalue를 100번 더해서 86500처럼 값이 큼.
  }
  if ( AddADCvalue < minvaluex100)      //자동으로 AddADCvalue의 최소값을 계산
  {
    minvaluex100=AddADCvalue;
  }
  
  if ( AddADCvalue > maxvaluex100)      //자동으로 AddADCvalue의 최대값을 계산
  {
    maxvaluex100=AddADCvalue;
  }
 
  angle = (float)(AddADCvalue-minvaluex100)*360/(maxvaluex100-minvaluex100);   //소수점 두자리까지 계산하기 위해 float형으로 변환해서 계산
  

  if (angle < 100 ){
    if (angle < 10){   

  Serial.print(minvaluex100);
  Serial.print(" ");
  Serial.print(maxvaluex100);
  Serial.print(" ");
  Serial.print(AddADCvalue);
  Serial.print(" ");
  Serial.print(angle);
  Serial.println();
    }
    else{                           //100<angle<10이면 lcd에 값을 출력하기

  Serial.print(minvaluex100);
  Serial.print(" ");
  Serial.print(maxvaluex100);
  Serial.print(" ");
  Serial.print(AddADCvalue);
  Serial.print(" ");
  Serial.print(angle);
  Serial.println();  
  }
    
  }
  else{                               //angle>100이면 lcd에 값을 출력하기


  Serial.print(minvaluex100);
  Serial.print(" ");
  Serial.print(maxvaluex100);
  Serial.print(" ");
  Serial.print(AddADCvalue);
  Serial.print(" ");
  Serial.print(angle);
  Serial.println();  
  }
}
