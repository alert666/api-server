#!/usr/bin/env bash
# gen-certs.sh — 管理 CA / 服务端 / 客户端证书，支持按需单独生成
#
# 用法:
#   bash gen-certs.sh         完整生成（CA + server + client）
#   bash gen-certs.sh ca      仅生成 CA（已存在则跳过）
#   bash gen-certs.sh server  仅生成 / 续签服务端证书
#   bash gen-certs.sh client  仅生成 / 续签客户端证书

set -euo pipefail

CA_CN="alert666 CA"
SERVER_CN="alert666 api-server"
CLIENT_CN="alert666 agent"
DAYS=3650
KEY_BITS=2048

cd "$(dirname "$0")"
mkdir -p certs

generate_ca() {
  cd certs
  if [[ -f ca.key && -f ca.crt ]]; then
    echo "==> [CA] 已存在（ca.key + ca.crt），跳过"
  else
    echo "==> [CA] 生成私钥和自签名证书"
    openssl req -x509 -newkey rsa:${KEY_BITS} -days ${DAYS} -nodes \
      -keyout ca.key -out ca.crt \
      -subj "/CN=${CA_CN}" \
      -addext "basicConstraints=critical,CA:TRUE" \
      -addext "keyUsage=critical,keyCertSign,cRLSign"
  fi
  cd ..
}

generate_server() {
  cd certs
  if [[ ! -f ca.key || ! -f ca.crt ]]; then
    echo "==> [Server] CA 不存在，请先生成: bash gen-certs.sh ca"
    exit 1
  fi
  echo "==> [Server] 生成私钥和证书"
  openssl genrsa -out server.key ${KEY_BITS}
  openssl req -new -key server.key -out server.csr -subj "/CN=${SERVER_CN}"
  cat > server.ext <<EOF
basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:localhost,IP:127.0.0.1
EOF
  openssl x509 -req -in server.csr \
    -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out server.crt -days ${DAYS} -extfile server.ext
  rm -f server.csr server.ext
  echo "==> [Server] 证书已更新: server.crt + server.key"
  cd ..
}

generate_client() {
  cd certs
  if [[ ! -f ca.key || ! -f ca.crt ]]; then
    echo "==> [Client] CA 不存在，请先生成: bash gen-certs.sh ca"
    exit 1
  fi
  echo "==> [Client] 生成私钥和证书"
  openssl genrsa -out client.key ${KEY_BITS}
  openssl req -new -key client.key -out client.csr -subj "/CN=${CLIENT_CN}"
  cat > client.ext <<EOF
basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=clientAuth
EOF
  openssl x509 -req -in client.csr \
    -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out client.crt -days ${DAYS} -extfile client.ext
  rm -f client.csr client.ext ca.srl
  echo "==> [Client] 证书已更新: client.crt + client.key"
  cd ..
}

case "${1:-all}" in
  ca)     generate_ca ;;
  server) generate_server ;;
  client) generate_client ;;
  all)
    generate_ca
    generate_server
    generate_client
    ;;
  *)
    echo "用法: $0 [ca|server|client|all]"
    echo ""
    echo "  ca      仅生成 CA（如已存在则跳过）"
    echo "  server  仅生成 / 续签服务端证书"
    echo "  client  仅生成 / 续签客户端证书"
    echo "  all     完整生成（默认）"
    exit 1
    ;;
esac

echo ""
echo "==> certs/ 文件清单："
ls -1 certs/
