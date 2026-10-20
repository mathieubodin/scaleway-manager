# scaleway-manager chart

Installs the scaleway-manager server: one pod, a Service, and an HTTPRoute attached to a Gateway you name.

## Install

```bash
helm install scaleway-manager charts/scaleway-manager \
  --namespace scaleway-manager --create-namespace \
  --set httpRoute.gateway.name=<gateway> \
  --set httpRoute.gateway.namespace=<gateway-namespace> \
  --set image.tag=<tag>
```

- `httpRoute.gateway.name` and `httpRoute.gateway.namespace` are required; rendering fails without them.
- `image.tag` defaults to the chart `appVersion`, `0.0.0`, for which no image is published before #18: set it until then.
- Optional: `httpRoute.gateway.sectionName` (the HTTPS listener) and `httpRoute.hostnames`.

## One install per cluster

The ClusterRoles `scaleway-manager-namespace` and `scaleway-manager-cluster` have fixed names, so a second release in the same cluster conflicts with the first.

## No API rights

No role is bound: both ClusterRoles have no rule and no binding, and the pod's ServiceAccount does not mount its token.
