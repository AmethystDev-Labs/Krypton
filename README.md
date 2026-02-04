<!-- markdownlint-disable MD033 MD041 -->

<p align="center">
  <img src="" width="160" height="160" alt="Krypton" />
</p>

<div align="center">

# Krypton

_高性能 HTTP 反向代理网关，支持健康检查、加权负载、重试策略与可脚本化行为。_

</div>

<p align="center">
  <a href="https://github.com/AmethystDev-Labs/Krypton/actions/workflows/build.yml">
    <img src="https://github.com/AmethystDev-Labs/Krypton/actions/workflows/build.yml/badge.svg" alt="build" />
  </a>
  <a href="https://raw.githubusercontent.com/AmethystDev-Labs/Krypton/master/LICENSE">
    <img src="https://img.shields.io/badge/License-GPLv3-blue.svg" alt="license" />
  </a>
  <a href="https://github.com/AmethystDev-Labs/Krypton/releases">
    <img src="https://img.shields.io/github/v/release/AmethystDev-Labs/Krypton?include_prereleases" alt="release" />
  </a>
  <img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white" alt="go" />
</p>

<p align="center">
  <a href="docs/README.md">Docs (EN)</a>
  ·
  <a href="docs/zh_cn/README.md">中文文档</a>
  ·
  <a href="https://github.com/AmethystDev-Labs/Krypton/releases">Releases</a>
  ·
  <a href="https://github.com/AmethystDev-Labs/Krypton/issues">Issues</a>
</p>

## 简介

Krypton 是一个高性能 HTTP 反向代理网关，用于将请求分发到多个上游节点。
它包含健康检查、加权调度（SWRR）、失败降权、慢恢复与脚本扩展能力。
当启用 Admin API 后，可通过 `/.krypton/` 进行管理操作。

## 特性

- 加权负载均衡与分片锁，降低高并发下的锁竞争
- 主动与被动健康检查融合，快速降级、渐进恢复
- 重试策略可配置，支持超时与 5xx 重试
- Starlark 脚本用于健康检查与触发逻辑
- 可选 Admin API 用于运行期管理

## 快速开始

1. 复制配置模板。

```powershell
Copy-Item example.config.toml config.toml
```

2. 修改 `config.toml` 中的 `[[nodes]]` 地址与权重。

3. 启动服务。

```powershell
go run .
```

## 安装

方式一：从 GitHub Releases 下载预编译二进制（包含预发布版本）。

方式二：源码构建。

```powershell
Copy-Item example.config.toml config.toml
go build -o krypton.exe .
```

## 配置

- 配置文件：`config.toml`
- 模板文件：`example.config.toml`

更多配置说明请查看：
- `docs/en_us/config.md`
- `docs/zh_cn/config.md`

## 运行参数

- `KRYPTON_LOG_LEVEL=debug|info|warn|error`

## 支持

- 问题反馈：GitHub Issues

## 贡献

- 建议使用 `gofmt` 格式化代码。
- 提交前运行 `go vet ./...` 与 `go test ./...`。

## 许可证

GPL-3.0。详见 `LICENSE`。

## 项目状态

Pre-release 阶段，接口与行为可能会在稳定版前调整。