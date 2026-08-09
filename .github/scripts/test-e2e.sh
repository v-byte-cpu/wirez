#!/usr/bin/env bash
set -euo pipefail

readonly go_image="golang:1.26.5-alpine"
readonly gost_image="gogost/gost:3.2.6"
readonly origin_image="busybox:1.37.0"
readonly platform="linux/amd64"
readonly marker="wirez-e2e-ok"

repo_root="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
suffix="$$"
network_name="wirez-e2e-network-${suffix}"
artifact_volume="wirez-e2e-artifact-${suffix}"
origin_container="wirez-e2e-origin-${suffix}"
proxy_container="wirez-e2e-gost-${suffix}"

cleanup() {
	status=$?
	trap - EXIT

	if ((status != 0)); then
		printf '\nE2E failed; fixture logs follow.\n' >&2
		docker logs "${origin_container}" 2>&1 || true
		docker logs "${proxy_container}" 2>&1 || true
	fi

	docker rm -f "${origin_container}" "${proxy_container}" >/dev/null 2>&1 || true
	docker network rm "${network_name}" >/dev/null 2>&1 || true
	docker volume rm "${artifact_volume}" >/dev/null 2>&1 || true

	exit "${status}"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

if ! docker info >/dev/null 2>&1; then
	printf 'Docker is unavailable. Start Docker and try again.\n' >&2
	exit 1
fi

docker network create "${network_name}" >/dev/null
docker volume create "${artifact_volume}" >/dev/null

docker run -d --rm \
	--name "${origin_container}" \
	--network "${network_name}" \
	--platform "${platform}" \
	"${origin_image}" \
	sh -c 'mkdir -p /www; printf %s wirez-e2e-ok > /www/marker; exec httpd -f -p 8080 -h /www' \
	>/dev/null

docker run -d --rm \
	--name "${proxy_container}" \
	--network "${network_name}" \
	--platform "${platform}" \
	"${gost_image}" \
	-L socks5://:1080 \
	>/dev/null

origin_ip="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${origin_container}")"
proxy_ip="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "${proxy_container}")"

docker run --rm \
	--platform "${platform}" \
	--env CGO_ENABLED=0 \
	--env GOARCH=amd64 \
	--env GOFLAGS=-buildvcs=false \
	--mount "type=bind,src=${repo_root},dst=/workspace,readonly" \
	--mount "type=volume,src=${artifact_volume},dst=/out" \
	--mount type=volume,src=wirez-go-1.26.5-mod,dst=/go/pkg/mod \
	--mount type=volume,src=wirez-go-1.26.5-linux-amd64-build,dst=/root/.cache/go-build \
	--workdir /workspace \
	"${go_image}" \
	go build -trimpath -ldflags '-w -s' -o /out/wirez .

wirez_args=(
	--rm
	--network "${network_name}"
	--platform "${platform}"
	--cap-add NET_ADMIN
	--cap-add SYS_ADMIN
	--device /dev/net/tun
	--mount "type=volume,src=${artifact_volume},dst=/opt/wirez,readonly"
	"${go_image}"
	/opt/wirez/wirez run
)

response="$(
	docker run \
		"${wirez_args[@]}" \
		-F "${proxy_ip}:1080" \
		-- wget -qO- -T 10 "http://${origin_ip}:8080/marker"
)"

if [[ "${response}" != "${marker}" ]]; then
	printf 'Expected marker %q, got %q.\n' "${marker}" "${response}" >&2
	exit 1
fi

if docker run \
	"${wirez_args[@]}" \
	-F "${proxy_ip}:1081" \
	-- wget -qO- -T 10 "http://${origin_ip}:8080/marker" \
	>/dev/null 2>&1; then
	printf 'Negative control unexpectedly bypassed the unavailable SOCKS5 proxy.\n' >&2
	exit 1
fi

printf 'wirez TCP socksifier E2E passed (positive route and negative control).\n'
