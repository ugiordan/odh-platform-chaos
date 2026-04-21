# odh-model-controller Custom Experiments

This page provides templates for writing custom chaos experiments targeting odh-model-controller.


## odh-model-controller

```yaml
apiVersion: chaos.operatorchaos.io/v1alpha1
kind: ChaosExperiment
metadata:
  name: odh-model-controller-custom
spec:
  target:
    operator: odh-model-controller
    component: odh-model-controller
  steadyState:
    checks:
      - type: conditionTrue
        apiVersion: apps/v1
        kind: Deployment
        name: odh-model-controller
        namespace: opendatahub
        conditionType: Available
    timeout: "60s"
  injection:
    type: PodKill  # Change to desired injection type
    parameters:
      labelSelector: app=odh-model-controller
    ttl: "300s"
  hypothesis:
    description: >-
      Describe the expected behavior after fault injection.
    recoveryTimeout: 120s
```


## Running Custom Experiments

1. Save your experiment YAML to a file
2. Run: `operator-chaos run <file>`
3. Check results: `operator-chaos report .`

<!-- custom-start: examples -->
<!-- custom-end: examples -->
