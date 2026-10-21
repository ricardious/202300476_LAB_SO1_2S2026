SHELL := /bin/bash
.NOTPARALLEL:
.DEFAULT_GOAL := help
USERS ?= 5
DURATION ?= 30s
.PHONY: setup compare resilience setup-load help bootstrap mirror build up tunnels status doctor test e2e load load-ui artifact evidence down credentials proto dapr-on dapr-off dapr-test compare-flows
setup: bootstrap setup-load mirror build up
help:
	@echo 'setup | bootstrap | mirror | build | up | tunnels | status | doctor | test | e2e | dapr-test | dapr-on | dapr-off | compare-flows | load | load-ui | artifact | evidence | down'
setup-load:
	@bash scripts/setup-load.sh
bootstrap:
	@source scripts/env.sh; python3 scripts/manage.py bootstrap
mirror:
	@source scripts/env.sh; python3 scripts/manage.py mirror
build:
	@bash scripts/build.sh
up:
	@source scripts/env.sh; python3 scripts/manage.py bootstrap; python3 scripts/manage.py deploy; python3 scripts/manage.py tunnels; python3 scripts/manage.py locust-ui
tunnels:
	@source scripts/env.sh; python3 scripts/manage.py tunnels
status:
	@source scripts/env.sh; kubectl get nodes; k get pods,gateway,httproute,hpa,valkeyinstances,grafanainstances
doctor:
	@source scripts/env.sh; python3 scripts/doctor.py
test:
	@source scripts/env.sh; cd services; go test -p 1 ./...
	@source scripts/env.sh; cd api-rust; cargo test --locked -j 1
	@source scripts/env.sh; helm lint operator/helm-charts/valkeyinstance; helm lint operator/helm-charts/grafanainstance
compare: artifact
	@bash scripts/compare.sh
resilience:
	@source scripts/env.sh; python3 tests/resilience.py
e2e:
	@source scripts/env.sh; python3 tests/e2e.py
dapr-test:
	@source scripts/env.sh; python3 tests/dapr_e2e.py
dapr-on:
	@source scripts/env.sh; kubectl apply -f deploy/dapr.yaml; k rollout restart deployment/go-dapr; k rollout status deployment/go-dapr --timeout=180s
dapr-off:
	@source scripts/env.sh; k scale deployment/go-dapr --replicas=0
compare-flows: artifact
	@bash scripts/compare-flows.sh
artifact:
	@source scripts/env.sh; bash scripts/artifact.sh
load: artifact
	@source scripts/env.sh; SO1_LOAD_PROFILE="$$SO1_P3_DATA/runtime/input/load-profile.json" "$$SO1_P3_DATA/venv/bin/locust" -f load/locustfile.py --headless -u $(USERS) -r 1 -t $(DURATION) --host "http://127.0.0.1:$$SO1_P3_GATEWAY_PORT" --csv docs/evidencias/locust-manual --only-summary
load-ui: artifact
	@source scripts/env.sh; python3 scripts/manage.py locust-ui
evidence:
	@source scripts/env.sh; python3 scripts/evidence.py
down:
	@source scripts/env.sh; python3 scripts/manage.py stop
credentials:
	@source scripts/env.sh; python3 -c 'import json,os,pathlib; s=json.loads((pathlib.Path(os.environ["SO1_P3_DATA"])/"runtime/secrets.json").read_text()); print("Usuario: so1\nGrafana:",s["grafana_password"],"\nZot:",s["registry_password"])'
proto:
	@bash scripts/proto.sh

.PHONY: test-console
test-console:
	node --test tests/console.test.cjs
