# arkstudy Helm Chart

This chart deploys arkstudy microservices as a set of Deployments and Services.

## Structure
- Chart.yaml
- values.yaml
- templates/
  - _helpers.tpl
  - deployments.yaml
  - services.yaml
  - servicemonitors.yaml (optional if Prometheus Operator CRDs exist)
  - serviceaccounts.yaml (optional per-service SA)
  - hpa.yaml (optional autoscaling)
  - pdb.yaml (optional disruption budget)

## Quickstart
Install or upgrade:

```sh
# install to namespace arkstudy
helm upgrade --install arkstudy deploy/helm/arkstudy -n arkstudy --create-namespace

# with custom values
helm upgrade --install arkstudy deploy/helm/arkstudy -n arkstudy -f deploy/helm/arkstudy/values.yaml
```

Uninstall:

```sh
helm uninstall arkstudy -n arkstudy
```

## Configure services
Example snippet:

```yaml
services:
  gateway:
    enabled: true
    image: rigelnana/arkstudy-gateway:latest
    replicas: 1
    service:
      type: ClusterIP
      port: 8080
    containerPort: 8080
    envFrom:
      - configMapRef: { name: gateway-config }
      - secretRef: { name: gateway-secret }
    serviceMonitorEnabled: true
    # Advanced:
    resources:
      requests: { cpu: 100m, memory: 128Mi }
      limits: { cpu: 500m, memory: 512Mi }
    nodeSelector: { kubernetes.io/os: linux }
    tolerations: []
    affinity: {}
    serviceAnnotations: { prometheus.io/scrape: "true" }
    serviceAccount:
      create: true
    autoscaling:
      enabled: true
      minReplicas: 1
      maxReplicas: 3
      targetCPUUtilizationPercentage: 70
    pdb:
      enabled: true
      minAvailable: 1

  llm-service:
    enabled: true
    image: rigelnana/arkstudy-llm-service:latest
    service:
      port: 8000
    containerPort: 8000
    envFrom:
      - secretRef: { name: openai-secret }
    # ServiceMonitor customizations
    serviceMonitorEnabled: true
    serviceMonitorPortName: http
    serviceMonitorInterval: 15s
```

## Notes
- labels/selectors unify on app.kubernetes.io/name/instance/component
- You can disable any microservice by setting `enabled: false` under `services.<name>`
- ServiceMonitor requires kube-prometheus-stack or Prometheus Operator CRDs present in the cluster
