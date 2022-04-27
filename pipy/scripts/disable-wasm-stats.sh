#!/bin/bash

set -euo pipefail

if [ -z "$1" ]; then
  echo "Error: expected one argument OSM_HOME"
  exit 1
fi

OSM_HOME=$1

sed -i 's/enableWASMStats: true$/enableWASMStats: false/g' ${OSM_HOME}/charts/osm/values.yaml

sed -i 's/^FROM --platform=\$BUILDPLATFORM \(.*\)openservicemesh\/proxy-wasm-cpp-sdk/#FROM --platform=\$BUILDPLATFORM \1openservicemesh\/proxy-wasm-cpp-sdk/g' ${OSM_HOME}/dockerfiles/Dockerfile.osm-controller
sed -i 's/^WORKDIR \/wasm/#WORKDIR \/wasm/g' ${OSM_HOME}/dockerfiles/Dockerfile.osm-controller
sed -i 's/^COPY \.\/wasm \./#COPY \.\/wasm \./g' ${OSM_HOME}/dockerfiles/Dockerfile.osm-controller
sed -i 's/^RUN \/build_wasm\.sh/#RUN \/build_wasm\.sh/g' ${OSM_HOME}/dockerfiles/Dockerfile.osm-controller
sed -i 's/^COPY --from=wasm/#COPY --from=wasm/g' ${OSM_HOME}/dockerfiles/Dockerfile.osm-controller
