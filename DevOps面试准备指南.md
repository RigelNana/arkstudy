# DevOps 面试准备指南 - ArkStudy 项目

## 项目概述

ArkStudy 是一个基于微服务架构的智能学习平台，采用现代化的 DevOps 技术栈实现全自动化的开发、测试、部署和运维流程。

### 技术栈概览
- **容器化**: Docker + Kubernetes
- **编排工具**: Helm + Kustomize
- **CI/CD**: GitLab CI/CD
- **镜像构建**: Kaniko
- **服务网格**: 原生 Kubernetes Service
- **存储**: PostgreSQL + pgvector, MinIO, Redis
- **消息队列**: Kafka (KRaft 模式)
- **监控**: Prometheus + Grafana
- **日志**: 标准化结构日志
- **配置管理**: ConfigMap + Secret

## 一、容器化架构与实践

### 1.1 Docker 多阶段构建优化

**Q: 请介绍你们项目的容器化策略？**

A: 我们项目采用了多语言微服务架构，针对不同服务类型设计了优化的容器化策略：

**Go 服务 Dockerfile 示例 (Gateway/Auth/User/Material服务):**
```dockerfile
# 多阶段构建 - 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# 运行阶段 - 最小化镜像
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/config ./config
EXPOSE 8080
CMD ["./main"]
```

**Python 服务 Dockerfile 示例 (LLM服务):**
```dockerfile
FROM python:3.11-slim
WORKDIR /app

# 系统依赖
RUN apt-get update && apt-get install -y \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Python 依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .
EXPOSE 8000 50054
CMD ["python", "-m", "app.main"]
```

**优化策略:**
1. **多阶段构建**: 分离构建环境和运行环境，减少镜像大小
2. **基础镜像选择**: Go 使用 alpine，Python 使用 slim 变体
3. **依赖层缓存**: 优先复制依赖文件，利用 Docker 层缓存
4. **安全加固**: 非 root 用户运行，最小化包安装

### 1.2 Docker Compose 本地开发环境

**Q: 如何管理复杂的微服务本地开发环境？**

A: 我们使用 Docker Compose 编排完整的本地开发环境，包含 8 个微服务和 5 个基础设施组件：

```yaml
services:
  # 应用服务
  gateway:
    build:
      context: ..
      dockerfile: gateway/Dockerfile
    ports:
      - "8080:8080"
    depends_on: 
      - user-service
      - auth-service
      - material-service
      - llm-service

  # 基础设施
  db:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: RigelNana77
      POSTGRES_DB: arkdb
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d arkdb"]
      interval: 5s
      timeout: 5s
      retries: 5

  kafka:
    image: confluentinc/cp-kafka:latest
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093
      # KRaft 模式配置，无需 Zookeeper
```

**关键特性:**
- **健康检查**: 所有基础设施服务配置健康检查
- **依赖管理**: 使用 depends_on + condition 确保启动顺序
- **环境隔离**: 通过 .env 文件管理不同环境配置
- **数据持久化**: 关键数据使用 named volume

## 二、Kubernetes 生产部署

### 2.1 微服务 Kubernetes 架构

**Q: 描述你们在 Kubernetes 上的微服务部署架构？**

A: 我们的 Kubernetes 架构采用命名空间隔离、多层负载均衡的设计：

```yaml
# 架构层次
arkstudy-dev/prod namespace
├── Gateway (NodePort/LoadBalancer)
├── Auth Service (ClusterIP)
├── User Service (ClusterIP) 
├── Material Service (ClusterIP)
├── LLM Service (ClusterIP)
├── OCR Service (ClusterIP)
├── Infrastructure
│   ├── PostgreSQL (StatefulSet)
│   ├── Redis (Deployment)
│   ├── MinIO (StatefulSet)
│   └── Kafka (StatefulSet)
```

**网络流量路径:**
```
Internet → LoadBalancer → Gateway Pod → Internal Services
                     ↓
              Service Discovery (DNS)
                     ↓
              Backend Microservices
```

### 2.2 Kustomize 环境管理

**Q: 如何管理多环境的 Kubernetes 配置？**

A: 我们使用 Kustomize 实现 Base + Overlay 模式：

```yaml
# k8s/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: arkstudy
resources:
  - namespace.yaml
  - postgres.yaml
  - auth-service.yaml
  - user-service.yaml
  # ... 其他基础资源

# k8s/overlays/dev/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
patchesStrategicMerge:
  - deployment-patches.yaml
  - service-patches.yaml
replicas:
  - name: auth-service
    count: 1
images:
  - name: rigelnana/arkstudy-auth-service
    newTag: dev-latest
```

**环境差异化配置:**
- **Dev**: 单副本、小资源限制、NodePort 暴露
- **Prod**: 多副本、严格资源限制、LoadBalancer + TLS

### 2.3 Helm Chart 包管理

**Q: 为什么选择 Helm，如何设计 Chart 结构？**

A: Helm 提供了更好的模板化和版本管理能力，我们设计了灵活的 Chart 结构：

```yaml
# deploy/helm/arkstudy/values.yaml
services:
  gateway:
    enabled: true
    image: "rigelnana/arkstudy-gateway:latest"
    replicas: 1
    service:
      type: ClusterIP
      port: 8080
    resources:
      limits:
        cpu: 500m
        memory: 512Mi
    config:
      LOG_LEVEL: "info"
    secrets:
      JWT_SECRET: "your-secret"

monitoring:
  enabled: true
  scrapeInterval: 15s
```

**Chart 模板设计:**
```yaml
# templates/deployment.yaml
{{- range $name, $service := .Values.services }}
{{- if $service.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "arkstudy.fullname" $ }}-{{ $name }}
spec:
  replicas: {{ $service.replicas | default 1 }}
  template:
    spec:
      containers:
      - name: {{ $name }}
        image: {{ $service.image }}
        resources: {{- toYaml $service.resources | nindent 10 }}
{{- end }}
{{- end }}
```

## 三、CI/CD 流水线设计

### 3.1 GitLab CI/CD 多阶段流水线

**Q: 介绍你们的 CI/CD 流水线设计思路？**

A: 我们设计了 7 阶段的 GitLab CI/CD 流水线，确保代码质量和部署安全：

```yaml
stages:
  - lint      # 代码规范检查
  - test      # 单元测试
  - build     # 构建验证
  - docker    # 镜像构建
  - validate  # 配置验证
  - deploy    # 部署

# 代码质量门禁
go:lint:
  stage: lint
  image: golang:1.22-alpine
  script:
    - go fmt ./... | tee /dev/stderr
    - go vet ./...
  rules:
    - if: $CI_PIPELINE_SOURCE =~ /^(push|merge_request_event)$/

python:lint-llm:
  stage: lint
  image: python:3.11-slim
  script:
    - cd services/llm-service
    - ruff check .
```

### 3.2 Kaniko 镜像构建

**Q: 为什么选择 Kaniko 而不是 Docker-in-Docker？**

A: Kaniko 提供了更安全、高效的容器化镜像构建方案：

```yaml
.kaniko-build-template:
  stage: docker
  image:
    name: gcr.io/kaniko-project/executor:latest
    entrypoint: [""]
  script:
    - |
      # 动态配置认证
      if [ -n "$DOCKER_USER" ] && [ -n "$DOCKER_PASSWORD" ]; then
        AUTH=$(printf "%s:%s" "$DOCKER_USER" "$DOCKER_PASSWORD" | base64 -w0)
        mkdir -p /kaniko/.docker
        cat > /kaniko/.docker/config.json <<EOF
        {"auths": {"https://index.docker.io/v1/": {"auth": "$AUTH"}}}
        EOF
      fi
    - |
      # 多标签构建策略
      DESTS="--destination $DOCKER_REGISTRY/$DOCKER_NAMESPACE/$IMAGE_NAME:$IMAGE_TAG"
      if [ "$CI_COMMIT_BRANCH" = "main" ]; then
        DESTS="$DESTS --destination $DOCKER_REGISTRY/$DOCKER_NAMESPACE/$IMAGE_NAME:latest"
      fi
      /kaniko/executor $DESTS \
        --context $CI_PROJECT_DIR/$BUILD_CONTEXT \
        --dockerfile $CI_PROJECT_DIR/$BUILD_CONTEXT/$DOCKERFILE \
        --snapshotMode=redo
```

**Kaniko 优势:**
1. **无需特权模式**: 提高安全性
2. **层缓存优化**: 提高构建速度
3. **多架构支持**: 支持 ARM64/AMD64
4. **内存优化**: 减少资源消耗

### 3.3 部署自动化策略

**Q: 如何实现安全的自动化部署？**

A: 我们实现了多层次的部署安全机制：

```yaml
# 配置验证阶段
kustomize:validate-dev:
  stage: validate
  image: ghcr.io/kubernetes-sigs/kustomize/kustomize:v5.4.2
  script:
    - kustomize build k8s/overlays/dev >/dev/null

helm:lint:
  stage: validate
  image: alpine/helm:3.15.3
  script:
    - helm lint deploy/helm/arkstudy
    - helm template test deploy/helm/arkstudy --namespace arkstudy >/dev/null

# 手动部署门禁
deploy:helm-dev:
  stage: deploy
  image: alpine/helm:3.15.3
  script:
    - helm upgrade --install arkstudy deploy/helm/arkstudy \
        -n arkstudy --create-namespace \
        -f deploy/helm/arkstudy/values-dev.yaml
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      when: manual  # 生产部署需要手动确认
```

## 四、监控与可观测性

### 4.1 Prometheus 监控栈

**Q: 如何设计微服务的监控体系？**

A: 我们构建了完整的 Prometheus 生态监控栈：

```bash
# 部署 kube-prometheus-stack
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --values monitoring-values.yaml
```

**监控架构:**
```yaml
# ServiceMonitor 配置
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: arkstudy-services
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app.kubernetes.io/part-of: arkstudy
  endpoints:
  - port: metrics
    path: /metrics
    interval: 15s
```

**关键监控指标:**
- **基础设施**: CPU、内存、磁盘、网络
- **应用性能**: 请求延迟、吞吐量、错误率
- **业务指标**: 用户活跃度、学习材料上传量
- **依赖服务**: PostgreSQL、Redis、Kafka 健康状态

### 4.2 日志管理策略

**Q: 如何管理微服务的日志？**

A: 我们采用结构化日志 + 中心化收集的策略：

```go
// Go 服务日志标准
import "github.com/sirupsen/logrus"

func setupLogger() *logrus.Logger {
    logger := logrus.New()
    logger.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: time.RFC3339,
    })
    logger.SetLevel(logrus.InfoLevel)
    return logger
}

// 结构化日志示例
logger.WithFields(logrus.Fields{
    "user_id":    userID,
    "request_id": requestID,
    "service":    "auth-service",
    "operation":  "login",
    "duration":   duration,
}).Info("User login successful")
```

**日志收集链路:**
```
应用容器 → stdout/stderr → Kubernetes logs API → Fluent Bit → Elasticsearch → Kibana
```

## 五、安全与运维最佳实践

### 5.1 Kubernetes 安全加固

**Q: 在 Kubernetes 中如何确保安全性？**

A: 我们实施了多层次的安全防护：

```yaml
# Pod Security Context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 2000
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true

# Network Policy
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: arkstudy-network-policy
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/part-of: arkstudy
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app.kubernetes.io/part-of: arkstudy
```

### 5.2 资源管理与自动扩缩容

**Q: 如何进行资源规划和自动扩缩容？**

A: 我们基于历史数据和业务特点设计了资源策略：

```yaml
# HPA 配置
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: arkstudy-gateway
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 六、故障排查与性能优化

### 6.1 常见问题排查流程

**Q: 如何快速定位和解决生产环境问题？**

A: 我们建立了标准化的故障排查流程：

```bash
# 1. 快速健康检查
kubectl get pods -n arkstudy-prod
kubectl get services -n arkstudy-prod
kubectl top pods -n arkstudy-prod

# 2. 日志分析
kubectl logs -n arkstudy-prod deployment/arkstudy-gateway --tail=100
kubectl logs -n arkstudy-prod -l app=auth-service --previous

# 3. 网络连通性测试
kubectl exec -n arkstudy-prod gateway-pod -- nslookup auth-service
kubectl exec -n arkstudy-prod gateway-pod -- curl -I http://auth-service:50051/health

# 4. 资源使用分析
kubectl describe pod -n arkstudy-prod auth-service-xxx
kubectl get events -n arkstudy-prod --sort-by='.lastTimestamp'
```

### 6.2 性能调优实践

**Q: 如何进行系统性能优化？**

A: 我们从多个维度进行性能优化：

**应用层优化:**
```go
// Go 服务优化
func main() {
    // 设置 GOMAXPROCS
    runtime.GOMAXPROCS(runtime.NumCPU())
    
    // 连接池优化
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(time.Hour)
}
```

**容器层优化:**
```yaml
resources:
  requests:
    cpu: 100m      # 保证基础性能
    memory: 128Mi
  limits:
    cpu: 500m      # 防止资源争夺
    memory: 512Mi
```

**集群层优化:**
- **节点选择**: 计算密集型服务调度到高性能节点
- **存储优化**: SSD 存储类用于数据库工作负载
- **网络优化**: 启用 CNI 网络加速插件

## 七、未来规划与技术演进

### 7.1 技术债务管理

**Q: 如何平衡新功能开发和技术债务？**

A: 我们建立了技术债务管理机制：

1. **定期评估**: 每季度技术债务 Review
2. **优先级排序**: 基于影响范围和修复成本
3. **渐进式重构**: 避免大爆炸式改造
4. **自动化测试**: 确保重构安全性

### 7.2 下一步技术升级计划

**近期计划 (3-6个月):**
- **Service Mesh**: 引入 Istio 增强服务治理
- **GitOps**: 采用 ArgoCD 实现声明式部署
- **安全扫描**: 集成 Trivy 镜像安全扫描

**中期计划 (6-12个月):**
- **多集群管理**: 实现跨地域高可用
- **Chaos Engineering**: 引入混沌工程提高系统韧性
- **成本优化**: 实施 Spot Instance 节约成本

## 八、面试高频问题与答案

### 8.1 容器化相关

**Q: Docker 和 Kubernetes 的区别是什么？**

A: Docker 是容器运行时，负责容器的创建和管理；Kubernetes 是容器编排平台，负责大规模容器集群的调度、管理和运维。在我们项目中，Docker 用于构建应用镜像，Kubernetes 用于生产环境的容器编排。

**Q: 如何优化 Docker 镜像大小？**

A: 我们采用了多种优化策略：
1. 多阶段构建分离构建和运行环境
2. 选择合适的基础镜像 (alpine/slim)
3. 合并 RUN 指令减少层数
4. 使用 .dockerignore 排除不必要文件
5. 清理包管理器缓存

### 8.2 Kubernetes 相关

**Q: Kubernetes 中 Service 的几种类型及使用场景？**

A: 
- **ClusterIP**: 集群内部通信，我们的微服务间通信使用
- **NodePort**: 暴露固定端口，开发环境使用
- **LoadBalancer**: 云环境负载均衡，生产环境网关使用
- **ExternalName**: 外部服务映射，数据库等外部依赖使用

**Q: 如何实现 Kubernetes 的滚动更新？**

A: 我们通过 Deployment 的 strategy 配置实现零停机更新：
```yaml
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 25%
    maxSurge: 25%
```

**Q: 解释 Kubernetes 的调度机制？**

A: Kubernetes 调度器通过以下步骤进行 Pod 调度：
1. **过滤阶段**: 根据资源需求、节点选择器、亲和性规则筛选候选节点
2. **评分阶段**: 基于资源利用率、数据局部性等指标对节点评分
3. **选择阶段**: 选择最高分节点进行调度

我们项目中的调度策略：
```yaml
# 资源请求调度
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

# 反亲和性避免单点故障
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app
            operator: In
            values:
            - gateway
        topologyKey: kubernetes.io/hostname
```

**Q: 如何处理 StatefulSet 与 Deployment 的区别？**

A: 在我们项目中的应用：

**Deployment (无状态服务)**:
```yaml
# Gateway、Auth、User 等无状态服务
apiVersion: apps/v1
kind: Deployment
metadata:
  name: arkstudy-gateway
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
  template:
    spec:
      containers:
      - name: gateway
        image: rigelnana/arkstudy-gateway:latest
```

**StatefulSet (有状态服务)**:
```yaml
# PostgreSQL、Redis 等有状态服务
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: arkstudy-postgres
spec:
  serviceName: postgres
  replicas: 1
  template:
    spec:
      containers:
      - name: postgres
        image: pgvector/pgvector:pg16
  volumeClaimTemplates:
  - metadata:
      name: postgres-data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: "fast-ssd"
      resources:
        requests:
          storage: 50Gi
```

**Q: 如何实现 Kubernetes 集群的高可用？**

A: 我们的高可用架构设计：

1. **控制平面高可用**:
```yaml
# 3个 Master 节点
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
controlPlaneEndpoint: "k8s-api.example.com:6443"
etcd:
  external:
    endpoints:
    - https://etcd1.example.com:2379
    - https://etcd2.example.com:2379
    - https://etcd3.example.com:2379
```

2. **应用层高可用**:
```yaml
# 多副本 + Pod 分布
apiVersion: apps/v1
kind: Deployment
spec:
  replicas: 3
  template:
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app
                operator: In
                values:
                - gateway
            topologyKey: "kubernetes.io/hostname"
```

**Q: Kubernetes 网络模型和 CNI 的理解？**

A: Kubernetes 网络遵循以下原则：
- 每个 Pod 都有独立的 IP
- Pod 间可以直接通信，无需 NAT
- Node 与 Pod 间可以直接通信

我们的网络架构：
```yaml
# Calico CNI 配置
apiVersion: projectcalico.org/v3
kind: NetworkPolicy
metadata:
  name: arkstudy-network-policy
spec:
  selector: app == 'arkstudy'
  types:
  - Ingress
  - Egress
  ingress:
  - action: Allow
    source:
      selector: app == 'arkstudy'
  egress:
  - action: Allow
    destination:
      nets:
      - 10.0.0.0/8  # 内部网络
```

### 8.3 CI/CD 相关

**Q: CI/CD 流水线失败如何快速定位问题？**

A: 我们的定位流程：
1. 查看 GitLab CI 作业日志
2. 检查阶段依赖关系
3. 验证环境变量和密钥配置
4. 测试本地构建复现问题
5. 回滚到上一个稳定版本

**Q: 如何保证部署的安全性？**

A: 多重安全机制：
1. 代码审查 + 自动化测试
2. 配置验证阶段
3. 手动部署门禁
4. 分阶段部署 (dev → staging → prod)
5. 快速回滚机制

**Q: 如何设计可扩展的 CI/CD 流水线？**

A: 我们采用了模板化和继承机制：

```yaml
# 基础模板定义
.go-build-template: &go-build
  image: golang:1.22-alpine
  before_script:
    - go env -w GOMODCACHE=$CI_PROJECT_DIR/.cache/gomod
    - mkdir -p .cache/go-build
  cache:
    key: "$CI_PROJECT_NAME-go-$CI_COMMIT_REF_SLUG"
    paths:
      - .cache/gomod
      - .cache/go-build

.kaniko-build-template: &kaniko-build
  stage: docker
  image:
    name: gcr.io/kaniko-project/executor:latest
    entrypoint: [""]
  variables:
    BUILD_CONTEXT: "."
    DOCKERFILE: "Dockerfile"

# 具体服务继承模板
go:test-auth:
  <<: *go-build
  script:
    - cd services/auth-service
    - go test ./...

docker:auth-service:
  <<: *kaniko-build
  variables:
    BUILD_CONTEXT: "services/auth-service"
    IMAGE_NAME: "arkstudy-auth-service"
```

**Q: 如何实现多环境的配置管理？**

A: 我们使用 GitLab CI/CD 变量 + 环境特定配置：

```yaml
# 环境变量配置
variables:
  DOCKER_REGISTRY: index.docker.io
  DOCKER_NAMESPACE: rigelnana
  
deploy:dev:
  environment:
    name: development
    url: https://dev.arkstudy.com
  variables:
    KUBE_NAMESPACE: arkstudy-dev
    HELM_VALUES_FILE: values-dev.yaml
  script:
    - helm upgrade --install arkstudy-dev ./deploy/helm/arkstudy \
        -n $KUBE_NAMESPACE \
        -f ./deploy/helm/arkstudy/$HELM_VALUES_FILE

deploy:prod:
  environment:
    name: production
    url: https://arkstudy.com
  variables:
    KUBE_NAMESPACE: arkstudy-prod
    HELM_VALUES_FILE: values-prod.yaml
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      when: manual
```

**Q: 如何处理微服务的依赖构建？**

A: 我们设计了智能的变更检测和依赖构建：

```yaml
# 变更检测脚本
.changes-detection: &changes-detection
  - |
    if [ "$CI_PIPELINE_SOURCE" = "merge_request_event" ]; then
      CHANGED_FILES=$(git diff --name-only $CI_MERGE_REQUEST_TARGET_BRANCH_SHA...$CI_COMMIT_SHA)
    else
      CHANGED_FILES=$(git diff --name-only $CI_COMMIT_BEFORE_SHA...$CI_COMMIT_SHA)
    fi
    
    # 检查特定服务是否有变更
    if echo "$CHANGED_FILES" | grep -q "^services/auth-service/"; then
      export BUILD_AUTH_SERVICE=true
    fi

# 条件构建
docker:auth-service:
  extends: .kaniko-build-template
  before_script:
    - *changes-detection
  rules:
    - if: $BUILD_AUTH_SERVICE == "true"
    - changes:
        - services/auth-service/**/*
        - proto/auth/**/*  # protobuf 变更也触发构建
```

**Q: 如何实现蓝绿部署？**

A: 我们通过 Kubernetes Service 切换实现蓝绿部署：

```yaml
# 蓝绿部署脚本
deploy:blue-green:
  stage: deploy
  script:
    - |
      # 获取当前活跃版本
      CURRENT_VERSION=$(kubectl get service arkstudy-gateway -o jsonpath='{.spec.selector.version}')
      if [ "$CURRENT_VERSION" = "blue" ]; then
        NEW_VERSION="green"
      else
        NEW_VERSION="blue"
      fi
      
      # 部署新版本
      helm upgrade --install arkstudy-$NEW_VERSION ./deploy/helm/arkstudy \
        --set image.tag=$CI_COMMIT_SHA \
        --set version=$NEW_VERSION \
        -n arkstudy-prod
      
      # 健康检查
      kubectl rollout status deployment/arkstudy-gateway-$NEW_VERSION -n arkstudy-prod
      
      # 切换流量
      kubectl patch service arkstudy-gateway -n arkstudy-prod \
        -p '{"spec":{"selector":{"version":"'$NEW_VERSION'"}}}'
      
      # 清理旧版本 (可选)
      sleep 300  # 等待5分钟确认稳定
      helm uninstall arkstudy-$CURRENT_VERSION -n arkstudy-prod
```

**Q: 如何实现回滚机制？**

A: 我们实现了多层次的回滚策略：

```yaml
# 快速回滚作业
rollback:
  stage: deploy
  image: alpine/helm:3.15.3
  script:
    - |
      # Helm 回滚
      helm rollback arkstudy -n arkstudy-prod
      
      # 验证回滚状态
      kubectl rollout status deployment/arkstudy-gateway -n arkstudy-prod
      
      # 镜像回滚 (如果需要)
      if [ -n "$ROLLBACK_IMAGE_TAG" ]; then
        kubectl set image deployment/arkstudy-gateway \
          gateway=rigelnana/arkstudy-gateway:$ROLLBACK_IMAGE_TAG \
          -n arkstudy-prod
      fi
  when: manual
  rules:
    - if: $CI_PIPELINE_SOURCE == "web"
```

**Q: 如何进行流水线性能优化？**

A: 我们的优化策略：

1. **并行化执行**:
```yaml
# 并行测试和构建
go:test:
  stage: test
  script:
    - go test -parallel 4 ./...

go:lint:
  stage: test  # 与测试并行
  script:
    - golangci-lint run ./...
```

2. **缓存优化**:
```yaml
cache:
  key: "$CI_PROJECT_NAME-$CI_COMMIT_REF_SLUG"
  paths:
    - .cache/go-build
    - .cache/gomod
    - node_modules/
  policy: pull-push
```

3. **增量构建**:
```yaml
# 基于变更的增量构建
.changes: &changes
  changes:
    - services/auth-service/**/*
    - proto/auth/**/*

docker:auth-service:
  rules:
    - <<: *changes
    - if: $CI_COMMIT_BRANCH == "main"
```

这份 DevOps 面试准备指南涵盖了我们项目的核心技术实践，从基础的容器化到复杂的 Kubernetes 编排，从 CI/CD 自动化到监控运维，展示了完整的 DevOps 技术栈和最佳实践。

### 8.4 监控与可观测性相关

**Q: 如何设计微服务的监控指标体系？**

A: 我们采用了 RED + USE 方法论设计监控指标：

**RED 方法 (面向用户的服务)**:
```yaml
# Prometheus 监控配置
- alert: HighErrorRate
  expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.1
  for: 5m
  annotations:
    summary: "High error rate detected"

- alert: HighLatency
  expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5
  for: 5m

- alert: LowThroughput
  expr: rate(http_requests_total[5m]) < 1
  for: 10m
```

**USE 方法 (基础设施资源)**:
```yaml
# 系统资源监控
- alert: HighCPUUsage
  expr: (1 - rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100 > 80
  for: 5m

- alert: HighMemoryUsage
  expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 85
  for: 5m

- alert: HighDiskUtilization
  expr: rate(node_disk_io_time_seconds_total[5m]) > 0.8
  for: 5m
```

**Q: 如何实现分布式链路追踪？**

A: 我们通过 OpenTelemetry + Jaeger 实现链路追踪：

```go
// Go 服务追踪集成
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func initTracing() {
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://jaeger-collector:14268/api/traces"),
    ))
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("auth-service"),
            semconv.ServiceVersion("v1.0.0"),
        )),
    )
    otel.SetTracerProvider(tp)
}

// HTTP 中间件
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tracer := otel.Tracer("auth-service")
        ctx, span := tracer.Start(c.Request.Context(), c.Request.URL.Path)
        defer span.End()
        
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

**Q: 如何实现日志聚合和分析？**

A: 我们构建了 ELK Stack 日志管道：

```yaml
# Fluent Bit 配置
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluent-bit-config
data:
  fluent-bit.conf: |
    [SERVICE]
        Flush         1
        Log_Level     info
        Daemon        off
        
    [INPUT]
        Name              tail
        Path              /var/log/containers/*arkstudy*.log
        Parser            docker
        Tag               kube.*
        
    [FILTER]
        Name                kubernetes
        Match               kube.*
        Kube_URL            https://kubernetes.default.svc:443
        
    [OUTPUT]
        Name            es
        Match           *
        Host            elasticsearch.logging.svc.cluster.local
        Port            9200
        Index           arkstudy-logs
        Logstash_Format On
```

**结构化日志格式**:
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "service": "auth-service",
  "trace_id": "abc123def456",
  "span_id": "789xyz",
  "user_id": "user-123",
  "operation": "login",
  "duration_ms": 150,
  "status": "success",
  "message": "User login successful"
}
```

**Q: 如何设计告警策略？**

A: 我们实现了分层告警机制：

```yaml
# Prometheus 告警规则
groups:
- name: arkstudy.rules
  rules:
  # P1 - 立即响应
  - alert: ServiceDown
    expr: up{job=~"arkstudy-.*"} == 0
    for: 1m
    labels:
      severity: critical
      priority: P1
    annotations:
      summary: "Service {{ $labels.job }} is down"
      
  # P2 - 5分钟内响应
  - alert: HighErrorRate
    expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
    for: 5m
    labels:
      severity: warning
      priority: P2
      
  # P3 - 工作时间响应
  - alert: ResourceUsageHigh
    expr: container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.8
    for: 15m
    labels:
      severity: info
      priority: P3
```

**告警路由配置**:
```yaml
# AlertManager 配置
route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'
  routes:
  - match:
      priority: P1
    receiver: 'pagerduty-critical'
  - match:
      priority: P2
    receiver: 'slack-warnings'
  - match:
      priority: P3
    receiver: 'email-notifications'
```

**Q: 如何进行容量规划？**

A: 我们基于历史数据和增长预测进行容量规划：

```prometheus
# 容量规划查询
# CPU 使用趋势
increase(container_cpu_usage_seconds_total[24h])

# 内存使用增长
predict_linear(container_memory_usage_bytes[7d], 30*24*3600)

# 请求量增长预测
predict_linear(rate(http_requests_total[1h])[7d], 30*24*3600)

# 存储增长
increase(container_fs_usage_bytes[24h])
```

**自动扩容配置**:
```yaml
# VPA (Vertical Pod Autoscaler)
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: arkstudy-gateway-vpa
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: arkstudy-gateway
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: gateway
      maxAllowed:
        cpu: 2
        memory: 4Gi
      minAllowed:
        cpu: 100m
        memory: 128Mi
```

### 8.5 安全与合规相关

**Q: 如何保证容器镜像的安全性？**

A: 我们实施了多层次的镜像安全策略：

```yaml
# Trivy 安全扫描
trivy:scan:
  stage: validate
  image: aquasec/trivy:latest
  script:
    - trivy image --exit-code 1 --severity HIGH,CRITICAL $IMAGE_NAME:$CI_COMMIT_SHA
  rules:
    - if: $CI_PIPELINE_SOURCE =~ /^(push|merge_request_event)$/

# 镜像签名验证
cosign:sign:
  stage: validate
  image: gcr.io/projectsigstore/cosign:latest
  script:
    - cosign sign --key cosign.key $IMAGE_NAME:$CI_COMMIT_SHA
```

**Q: 如何实现 RBAC 权限管理？**

A: 我们实现了细粒度的 Kubernetes RBAC：

```yaml
# 开发者权限
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: arkstudy-dev
  name: developer
rules:
- apiGroups: [""]
  resources: ["pods", "services", "configmaps"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]

# 生产环境只读权限
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: arkstudy-prod
  name: prod-reader
rules:
- apiGroups: [""]
  resources: ["pods", "services", "configmaps"]
  verbs: ["get", "list", "watch"]
```

这份 DevOps 面试准备指南涵盖了我们项目的核心技术实践，从基础的容器化到复杂的 Kubernetes 编排，从 CI/CD 自动化到监控运维，从安全合规到性能优化，展示了完整的 DevOps 技术栈和最佳实践。通过这些实际项目经验，可以充分展示在 DevOps 领域的技术深度和实践能力。