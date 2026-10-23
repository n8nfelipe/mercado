cluster_name = os.getenv("CLUSTER_NAME", "mercado-dev")

allow_k8s_contexts("kind-" + cluster_name)

# The runtime image intentionally has no process supervisor. Rebuild in place
# and terminate PID 1 so Kubernetes restarts the container with the new binary.
api_live_update = [
    sync("./backend", "/src"),
    run("mkdir -p /tmp/gocache && cd /src && GOCACHE=/tmp/gocache CGO_ENABLED=0 go build -o /app/api ./cmd/api && kill -TERM 1 || true"),
]

worker_live_update = [
    sync("./backend", "/src"),
    run("mkdir -p /tmp/gocache && cd /src && GOCACHE=/tmp/gocache CGO_ENABLED=0 go build -o /app/worker ./cmd/worker && kill -TERM 1 || true"),
]

web_live_update = [
    sync("./frontend", "/app/frontend"),
]

custom_build(
    "mercado-api:local",
    "docker build -t $EXPECTED_REF -f backend/Dockerfile.dev backend",
    deps=["backend"],
    live_update=api_live_update,
)

custom_build(
    "mercado-worker:local",
    "docker build -t $EXPECTED_REF -f backend/Dockerfile.dev backend",
    deps=["backend"],
    live_update=worker_live_update,
)

custom_build(
    "mercado-web:local",
    "docker build -t $EXPECTED_REF -f frontend/apps/web/Dockerfile.dev .",
    deps=["frontend", "package.json", "pnpm-lock.yaml", "pnpm-workspace.yaml"],
    live_update=web_live_update,
)

k8s_yaml(kustomize("deploy/k8s/overlays/local"))
k8s_resource("api", port_forwards=["18081:8080"], labels=["backend"])
k8s_resource("web", port_forwards=["3001:3000"], labels=["frontend"])
k8s_resource("worker", labels=["backend"])
