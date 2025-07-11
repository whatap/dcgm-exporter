# DCGM_FI_DEV_WEIGHTED_GPU_UTIL 메트릭 구현 완료

## 구현 개요

PRD 요구사항에 따라 NVIDIA MIG(Multi-Instance GPU) 환경에서 정확한 GPU 사용률 모니터링을 위한 `DCGM_FI_DEV_WEIGHTED_GPU_UTIL` 메트릭을 DCGM-Exporter에 성공적으로 구현했습니다.

## 변경된 파일들

### 1. `internal/pkg/counters/const.go`
```go
// 추가된 상수
DCGMExpWeightedGPUUtil = "DCGM_FI_DEV_WEIGHTED_GPU_UTIL"
```

### 2. `internal/pkg/counters/exporter_counters.go`
```go
// 추가된 ExporterCounter
DCGMWeightedGPUUtil ExporterCounter = iota + 9000

// String() 메서드에 케이스 추가
case DCGMWeightedGPUUtil:
    return DCGMExpWeightedGPUUtil

// DCGMFields 맵에 추가
DCGMWeightedGPUUtil.String(): DCGMWeightedGPUUtil,
```

### 3. `internal/pkg/collector/gpu_collector.go`
- **Import 추가**: `os`, `regexp` 패키지
- **메인 계산 함수**: `calculateWeightedGPUUtil()`
- **MIG 모드 처리**: `calculateMIGWeightedUtil()`
- **일반 GPU 처리**: `calculateNonMIGWeightedUtil()`
- **헬퍼 함수들**: `extractComputeSlices()`, `createWeightedGPUUtilMetric()` 등

## 구현된 기능

### MIG 모드 GPU (DCGM_FI_DEV_MIG_MODE="1")
```
가중치 GPU 사용률 = Σ(DCGM_FI_PROF_GR_ENGINE_ACTIVE × slice_ratio)
where: slice_ratio = compute_slices / DCGM_FI_DEV_MIG_MAX_SLICES
```

**예시 계산**:
- A100 GPU (max_slices=7)에서 6개의 1g.5gb MIG 인스턴스
- 각 인스턴스: engine_active × (1/7)
- 최종 결과: 모든 인스턴스의 가중치 합

### 일반 GPU (DCGM_FI_DEV_MIG_MODE="0")
```
가중치 GPU 사용률 = DCGM_FI_DEV_GPU_UTIL / 100
```

## 메트릭 출력 형식

```prometheus
# HELP DCGM_FI_DEV_WEIGHTED_GPU_UTIL Weighted GPU utilization for MIG and non-MIG devices
# TYPE DCGM_FI_DEV_WEIGHTED_GPU_UTIL gauge

# MIG GPU 예시
DCGM_FI_DEV_WEIGHTED_GPU_UTIL{
  gpu="1",
  UUID="GPU-9dadccd1-6248-ac2a-6e85-0af3fdfeef3c",
  device="nvidia1",
  modelName="NVIDIA A100-SXM4-40GB",
  DCGM_FI_DEV_MIG_MODE="1",
  calculation_method="weighted_sum"
} 0.322663

# 일반 GPU 예시
DCGM_FI_DEV_WEIGHTED_GPU_UTIL{
  gpu="0",
  UUID="GPU-d6215468-e63a-57fa-8e41-ef2ea1e698a5",
  device="nvidia0",
  modelName="NVIDIA A100-SXM4-40GB",
  DCGM_FI_DEV_MIG_MODE="0",
  calculation_method="direct"
} 0.770000
```

## 지원하는 MIG 프로필

- `1g.5gb` → 1 compute slice
- `2g.10gb` → 2 compute slices  
- `3g.20gb` → 3 compute slices
- `4g.20gb` → 4 compute slices
- `7g.40gb` → 7 compute slices

## 사용법

### 1. 빌드 및 배포
```bash
# 빌드
make build

# 또는 Docker 빌드
docker build -t dcgm-exporter:latest .
```

### 2. 실행
기존 DCGM-Exporter와 동일하게 실행하면 자동으로 새 메트릭이 포함됩니다:

```bash
./dcgm-exporter
```

### 3. Prometheus 쿼리 예시

**전체 클러스터의 평균 GPU 사용률**:
```promql
avg(DCGM_FI_DEV_WEIGHTED_GPU_UTIL)
```

**MIG 모드 GPU만 조회**:
```promql
DCGM_FI_DEV_WEIGHTED_GPU_UTIL{calculation_method="weighted_sum"}
```

**일반 GPU만 조회**:
```promql
DCGM_FI_DEV_WEIGHTED_GPU_UTIL{calculation_method="direct"}
```

**노드별 GPU 사용률**:
```promql
avg by (node) (DCGM_FI_DEV_WEIGHTED_GPU_UTIL)
```

## 검증 방법

### 1. 메트릭 확인
```bash
curl http://localhost:9400/metrics | grep DCGM_FI_DEV_WEIGHTED_GPU_UTIL
```

### 2. MIG 환경 테스트
```bash
# MIG 모드 활성화된 GPU에서 확인
nvidia-smi mig -lgi
curl http://localhost:9400/metrics | grep "calculation_method=\"weighted_sum\""
```

### 3. 계산 검증
실제 MIG 인스턴스들의 DCGM_FI_PROF_GR_ENGINE_ACTIVE 값들과 
계산된 DCGM_FI_DEV_WEIGHTED_GPU_UTIL 값이 올바른 가중치 비율로 계산되었는지 확인

## 기술적 특징

### 성능 최적화
- GPU UUID별로 그룹핑하여 효율적 처리
- 필요한 메트릭만 선별적으로 처리
- 메모리 사용량 최소화

### 에러 처리
- MIG 모드 감지 실패 시 기본값(0) 사용
- 파싱 에러 시 해당 인스턴스 스킵
- 필수 데이터 부족 시 메트릭 생성 안함

### 호환성
- 기존 DCGM-Exporter 기능과 완전 호환
- MIG + 일반 GPU 혼재 환경 지원
- 다양한 GPU 모델 지원 (A100, H100 등)

## 문제 해결

### 메트릭이 나타나지 않는 경우
1. MIG 모드 확인: `nvidia-smi -q | grep MIG`
2. DCGM_FI_PROF_GR_ENGINE_ACTIVE 메트릭 존재 확인
3. DCGM-Exporter 로그 확인

### 계산 값이 예상과 다른 경우
1. MIG 프로필 확인: `nvidia-smi mig -lgi`
2. DCGM_FI_DEV_MIG_MAX_SLICES 값 확인
3. 개별 MIG 인스턴스의 engine_active 값 확인

## 향후 개선 사항

1. **로깅 추가**: 계산 과정의 디버그 로그
2. **메트릭 검증**: 계산 결과의 유효성 검사
3. **성능 모니터링**: 계산 시간 측정 메트릭
4. **설정 옵션**: 계산 방식 커스터마이징

---

**구현 완료일**: 2025년 1월  
**구현자**: Assistant  
**검토 필요**: junnie  
**빌드 상태**: ✅ 성공