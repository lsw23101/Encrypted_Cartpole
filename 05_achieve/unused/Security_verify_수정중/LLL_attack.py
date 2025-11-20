import json
from fpylll import IntegerMatrix, LLL
from math import log2

# Load RLWE sample
with open("rlwe_sample.json", "r") as f:
    data = json.load(f)

a = data["a"]
b = data["b"]
q = int(data["q"])
N = int(data["N"])

# Construct lattice basis for the approximate CVP problem
B = IntegerMatrix(N+1, N+1)

for i in range(N):
    for j in range(N):
        B[i, j] = q if i == j else 0
    B[i, N] = a[i]

# Last row = target vector b
for j in range(N):
    B[N, j] = 0
B[N, N] = 1

print(">> Lattice Basis 준비 완료. LLL 시작...")

# LLL Reduction
LLL.reduction(B)

# s 추정
s_guess = [int(B[0, i]) for i in range(N)]

print(">> 추정된 s:", s_guess)