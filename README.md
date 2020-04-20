# kubernoisy

kubernoisy is a testing tool that creates/destroys kubernetes objects to simulate "churn" in a cluster. 
It also verifies creations and deletions by querying DNS records.

It produces metrics via Prometheus.

### Usage

```
Usage of ./kubernoisy:
  -namespace string
    	Namespace to operate in (default "load-test")
  -ops float
    	Operations per second (default 1)
  -prom string
    	Prometheus endpoint (default ":9696")
  -timeout duration
    	Timeout for validation (default 30s)
  -verbose
    	Verbose log output

```

### Metrics

* *kubernoisy_action_count_total{object, action}*: Counter of object actions.
* *kubernoisy_validation_fail_count_total{action}*: Counter of validation failures
* *kubernoisy_validation_duration_seconds{action}*: Delay to reflect in DNS record
