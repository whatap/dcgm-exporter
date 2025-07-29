# DCGM_FI_DEV_WEIGHTED_GPU_UTIL 메트릭 추가 PRD (요구사항 정의서)

## 1. 개요

### 1.1 목적
NVIDIA MIG(Multi-Instance GPU) 환경에서 정확한 GPU 사용률 모니터링을 위해 `DCGM_FI_DEV_WEIGHTED_GPU_UTIL` 메트릭을 DCGM-Exporter에 추가합니다.

### 1.2 배경
- MIG 모드에서는 기존 `DCGM_FI_DEV_GPU_UTIL` 메트릭이 지원되지 않음 (NVIDIA 공식 문서 명시)
- MIG 인스턴스별 `DCGM_FI_PROF_GR_ENGINE_ACTIVE` 값을 가중치 계산하여 전체 GPU 사용률 도출 필요
- PromQL 쿼리로 구현하기에는 복잡성과 성능 이슈가 있어 DCGM-Exporter 레벨에서 직접 구현

### 1.3 참조 문서
- [NVIDIA MIG 사용자 가이드](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/)
- [NVIDIA DCGM GitHub 이슈 #64](https://github.com/NVIDIA/DCGM/issues/64)
- [NVIDIA DCGM 공식 문서](https://docs.nvidia.com/datacenter/dcgm/latest/user-guide/feature-overview.html#understanding-metrics)

## 2. 요구사항

### 2.1 기능 요구사항

#### 2.1.1 메트릭 정의
- **메트릭 이름**: `DCGM_FI_DEV_WEIGHTED_GPU_UTIL`
- **메트릭 타입**: `gauge`
- **단위**: 비율 (0.0 ~ 1.0)
- **설명**: "Weighted GPU utilization for MIG and non-MIG devices"

#### 2.1.2 계산 로직

**MIG 모드 GPU (DCGM_FI_DEV_MIG_MODE=1):**
```
DCGM_FI_DEV_WEIGHTED_GPU_UTIL = Σ(DCGM_FI_PROF_GR_ENGINE_ACTIVE × slice_ratio)

where:
slice_ratio = compute_slices / DCGM_FI_DEV_MIG_MAX_SLICES
compute_slices = GPU_I_PROFILE에서 추출 (예: "1g.5gb" → 1, "2g.10gb" → 2)
```

**일반 GPU (DCGM_FI_DEV_MIG_MODE=0):**
```
DCGM_FI_DEV_WEIGHTED_GPU_UTIL = DCGM_FI_DEV_GPU_UTIL / 100
```

#### 2.1.3 실제 계산 예시
**A100 GPU (DCGM_FI_DEV_MIG_MAX_SLICES=7)에서 6개의 1g.5gb MIG 인스턴스:**
- MIG Instance 1: 0.982262 × (1/7) = 0.140323
- MIG Instance 2: 0.000002 × (1/7) = 0.000000
- MIG Instance 3: 0.510287 × (1/7) = 0.072898
- MIG Instance 4: 0.000003 × (1/7) = 0.000000
- MIG Instance 5: 0.766027 × (1/7) = 0.109432
- MIG Instance 6: 0.000069 × (1/7) = 0.000010
- **총합**: 0.322663 (32.27%)

### 2.2 기술 요구사항

#### 2.2.1 구현 위치
1. **메트릭 정의 추가**:
   - `internal/pkg/counters/exporter_counters.go`에 새로운 ExporterCounter 추가
   - `internal/pkg/counters/const.go`에 메트릭 이름 상수 추가

2. **계산 로직 구현**:
   - `internal/pkg/collector/gpu_collector.go`의 `toMetric` 함수 또는 새로운 함수에서 구현
   - MIG 모드 감지 및 가중치 계산 로직 추가

3. **메트릭 설정**:
   - `etc/default-counters.csv`에 새 메트릭 정의 추가 (선택사항)

#### 2.2.2 데이터 소스
- **MIG 모드 감지**: `DCGM_FI_DEV_MIG_MODE` 라벨
- **MIG 인스턴스 사용률**: `DCGM_FI_PROF_GR_ENGINE_ACTIVE` 메트릭
- **MIG 프로필 정보**: `GPU_I_PROFILE` 라벨 (예: "1g.5gb", "2g.10gb")
- **최대 슬라이스**: `DCGM_FI_DEV_MIG_MAX_SLICES` 라벨
- **일반 GPU 사용률**: `DCGM_FI_DEV_GPU_UTIL` 메트릭

#### 2.2.3 라벨 구조
```
DCGM_FI_DEV_WEIGHTED_GPU_UTIL{
  gpu="1",
  UUID="GPU-9dadccd1-6248-ac2a-6e85-0af3fdfeef3c",
  device="nvidia1",
  modelName="NVIDIA A100-SXM4-40GB",
  DCGM_FI_DEV_MIG_MODE="1",
  calculation_method="weighted_sum"  // 또는 "direct"
} 0.322663
```

### 2.3 성능 요구사항
- 기존 메트릭 수집 성능에 영향 최소화
- MIG 인스턴스 수에 비례한 선형 계산 복잡도
- 메모리 사용량 증가 최소화

### 2.4 호환성 요구사항
- 기존 DCGM-Exporter 기능과 완전 호환
- MIG 모드와 일반 모드 GPU 혼재 환경 지원
- 다양한 MIG 프로필 지원 (1g.5gb, 2g.10gb, 3g.20gb, 4g.20gb, 7g.40gb)
- A100, H100 등 다양한 GPU 모델 지원

## 3. 구현 가이드

### 3.1 파일별 수정 사항

#### 3.1.1 `internal/pkg/counters/const.go`
```go
const (
    // 기존 상수들...
    DCGMExpClockEventsCount = "DCGM_EXP_CLOCK_EVENTS_COUNT"
    DCGMExpXIDErrorsCount   = "DCGM_EXP_XID_ERRORS_COUNT"
    DCGMExpGPUHealthStatus  = "DCGM_EXP_GPU_HEALTH_STATUS"
    
    // 새로 추가
    DCGMExpWeightedGPUUtil  = "DCGM_FI_DEV_WEIGHTED_GPU_UTIL"
)
```

#### 3.1.2 `internal/pkg/counters/exporter_counters.go`
```go
const (
    DCGMFIUnknown           ExporterCounter = 0
    DCGMXIDErrorsCount      ExporterCounter = iota + 9000
    DCGMClockEventsCount    ExporterCounter = iota + 9000
    DCGMGPUHealthStatus     ExporterCounter = iota + 9000
    DCGMWeightedGPUUtil     ExporterCounter = iota + 9000  // 새로 추가
)

func (enm ExporterCounter) String() string {
    switch enm {
    case DCGMXIDErrorsCount:
        return DCGMExpXIDErrorsCount
    case DCGMClockEventsCount:
        return DCGMExpClockEventsCount
    case DCGMGPUHealthStatus:
        return DCGMExpGPUHealthStatus
    case DCGMWeightedGPUUtil:  // 새로 추가
        return DCGMExpWeightedGPUUtil
    default:
        return "DCGM_FI_UNKNOWN"
    }
}

var DCGMFields = map[string]ExporterCounter{
    DCGMXIDErrorsCount.String():   DCGMXIDErrorsCount,
    DCGMClockEventsCount.String(): DCGMClockEventsCount,
    DCGMGPUHealthStatus.String():  DCGMGPUHealthStatus,
    DCGMWeightedGPUUtil.String():  DCGMWeightedGPUUtil,  // 새로 추가
    DCGMFIUnknown.String():        DCGMFIUnknown,
}
```

#### 3.1.3 `internal/pkg/collector/gpu_collector.go`
새로운 함수 추가:
```go
// calculateWeightedGPUUtil calculates weighted GPU utilization for MIG and non-MIG devices
func calculateWeightedGPUUtil(metrics MetricsByCounter, d dcgm.Device, instanceInfo *deviceinfo.GPUInstanceInfo) (float64, error) {
    // MIG 모드 확인
    migMode := getMIGMode(d) // 구현 필요
    
    if migMode == 1 {
        // MIG 모드: 가중치 계산
        return calculateMIGWeightedUtil(metrics, d, instanceInfo)
    } else {
        // 일반 모드: GPU_UTIL 사용
        return calculateNonMIGUtil(metrics, d)
    }
}

func calculateMIGWeightedUtil(metrics MetricsByCounter, d dcgm.Device, instanceInfo *deviceinfo.GPUInstanceInfo) (float64, error) {
    // 1. 해당 GPU의 모든 MIG 인스턴스 찾기
    // 2. 각 인스턴스의 DCGM_FI_PROF_GR_ENGINE_ACTIVE 값 수집
    // 3. GPU_I_PROFILE에서 compute_slices 추출
    // 4. DCGM_FI_DEV_MIG_MAX_SLICES 값 확인
    // 5. 가중치 계산: Σ(engine_active × (compute_slices / max_slices))
    
    // 구현 세부사항은 기존 toMetric 함수 패턴 참조
}

func calculateNonMIGUtil(metrics MetricsByCounter, d dcgm.Device) (float64, error) {
    // DCGM_FI_DEV_GPU_UTIL 값을 0-1 범위로 변환 (기존 값은 0-100)
    // return gpu_util / 100.0
}
```

### 3.2 테스트 케이스

#### 3.2.1 단위 테스트
1. **MIG 모드 GPU 테스트**:
   - 다양한 MIG 프로필 조합 테스트
   - 가중치 계산 정확성 검증
   - 경계값 테스트 (0%, 100% 사용률)

2. **일반 GPU 테스트**:
   - GPU_UTIL 값 변환 테스트
   - 백분율 → 비율 변환 검증

3. **혼재 환경 테스트**:
   - MIG + 일반 GPU 동시 존재 시나리오

#### 3.2.2 통합 테스트
1. **실제 MIG 환경 테스트**:
   - A100 GPU에서 다양한 MIG 구성 테스트
   - 계산 결과와 실제 워크로드 상관관계 검증

2. **성능 테스트**:
   - 메트릭 수집 지연시간 측정
   - 메모리 사용량 모니터링

## 4. 검증 기준

### 4.1 기능 검증
- [ ] MIG 모드 GPU에서 가중치 계산이 정확히 수행됨
- [ ] 일반 GPU에서 기존 GPU_UTIL 값이 올바르게 변환됨
- [ ] 다양한 MIG 프로필 (1g.5gb, 2g.10gb 등) 지원
- [ ] 혼재 환경에서 각 GPU별로 올바른 계산 방식 적용

### 4.2 성능 검증
- [ ] 기존 메트릭 수집 성능 대비 5% 이내 성능 저하
- [ ] 메모리 사용량 10% 이내 증가
- [ ] MIG 인스턴스 수 증가에 따른 선형 성능 특성

### 4.3 호환성 검증
- [ ] 기존 DCGM-Exporter 기능 정상 동작
- [ ] 기존 메트릭 출력 형식 유지
- [ ] 다양한 GPU 모델에서 정상 동작

## 5. 배포 계획

### 5.1 개발 단계
1. **Phase 1**: 핵심 계산 로직 구현 및 단위 테스트
2. **Phase 2**: 통합 테스트 및 성능 최적화
3. **Phase 3**: 문서화 및 배포 준비

### 5.2 테스트 환경
- **개발 환경**: MIG 지원 GPU가 있는 테스트 클러스터
- **검증 환경**: 실제 운영과 유사한 혼재 환경

### 5.3 롤백 계획
- 기존 메트릭에 영향을 주지 않는 추가 메트릭이므로 롤백 위험 최소
- 문제 발생 시 해당 메트릭만 비활성화 가능

## 6. 참고 자료

### 6.1 기술 문서
- [MIG GPU 사용률 계산 방법론](/Users/jaeyoung/go/src/whatap-operator/docs/mig.md)
- [실제 데이터 예시](/Users/jaeyoung/go/src/whatap-operator/docs/eks-mig.csv)
- [OpenMetrics 형식 예시](/Users/jaeyoung/go/src/whatap-operator/docs/open.txt)

### 6.2 구현 참조
- 기존 DCGM-Exporter 메트릭 구현 패턴
- `internal/pkg/collector/gpu_collector.go`의 `toMetric` 함수
- `internal/pkg/counters/` 패키지의 메트릭 정의 방식

---

**작성자**: jaeyoung  
**작성일**: 2025년 1월  
**검토자**: junnie  
**승인자**: TBD