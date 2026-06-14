# 证书管理

gRPC mTLS 使用 `gen-certs.sh` 管理证书。

## 依赖

- OpenSSL（`openssl version` 验证）
- 建议在 Linux / macOS 或 WSL 下运行

## 使用

```bash
cd scripts

# 完整生成（首次部署，默认 SAN 含 localhost + 127.0.0.1）
bash gen-certs.sh

# 生成服务端证书，追加自定义 DNS/IP 到 SAN（避免 x509 hostname mismatch）
bash gen-certs.sh server "DNS:grpc-ops.suanlene.cn,IP:10.0.0.1"

# 仅生成/续签服务端证书
bash gen-certs.sh server

# 仅生成客户端证书（给新 agent）
bash gen-certs.sh client

# 仅生成 CA（已存在则跳过）
bash gen-certs.sh ca
```

> **关于 SAN**：`server` 子命令支持可选的第二个参数（逗号分隔的 `DNS:...` / `IP:...`），
> 会追加进证书的 `subjectAltName`。默认已含 `DNS:localhost,IP:127.0.0.1`。
> 客户端证书（`client`）不需要 SAN，因为 mTLS 中服务端不校验客户端主机名。

## 输出文件

所有文件生成到 `scripts/certs/` 下：

| 文件         | 用途       | 分发目标                          |
| ------------ | ---------- | --------------------------------- |
| `ca.key`     | CA 私钥    | **妥善保管，不要分发**            |
| `ca.crt`     | CA 证书    | 服务端和 agent 各存一份           |
| `server.crt` | 服务端证书 | api-server 的 `grpc.tls.certFile` |
| `server.key` | 服务端私钥 | api-server 的 `grpc.tls.keyFile`  |
| `client.crt` | 客户端证书 | agent 的 `grpc.tls.certFile`      |
| `client.key` | 客户端私钥 | agent 的 `grpc.tls.keyFile`       |

## 配置示例

**服务端 `config.yaml`：**

```yaml
grpc:
  bind: ":50051"
  tls:
    certFile: "/path/to/certs/server.crt"
    keyFile:  "/path/to/certs/server.key"
    caFile:   "/path/to/certs/ca.crt"
```

**Agent `config.yaml`：**

```yaml
grpc:
  tls:
    certFile: "/path/to/certs/client.crt"
    keyFile:  "/path/to/certs/client.key"
    caFile:   "/path/to/certs/ca.crt"
```

## 安全说明

- `ca.key` 是信任根，泄漏后攻击者可以签发任意证书连接你的服务端
- 建议 `ca.key` 离线存储，只在签发证书时挂载
- `gen-certs.sh server` / `client` 会覆盖旧的私钥和证书，旧证书立即失效