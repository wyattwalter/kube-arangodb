# Metrics

## Prometheus integration

### ArangoDeployment configuration

To be able to scrape metrics from ArangoDB Pods managed by Operator, we need to enable monitoring features designed for ArangoDeployment:

```yaml
spec:
  ...
  annotations:
    prometheus.io/scrape: 'true'
    prometheus.io/port: '9101'
  ...
  metrics:
    enabled: true
    tls: false
    mode: sidecar
    image: arangodb/arangodb-exporter:0.1.7
```
