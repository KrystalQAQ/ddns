# Cloudflare DDNS - 轻量级Docker容器

基于Go语言开发的轻量级Cloudflare DDNS自动更新工具，使用scratch基础镜像，最终镜像仅约2-3MB。

## 特点

- 🚀 **极小体积** - 基于scratch镜像，最终镜像仅2-3MB
- ⚡ **高性能** - Go语言编写，单个二进制文件
- 🔄 **自动更新** - 检测IP变化自动更新Cloudflare DNS
- 🐳 **容器化** - 开箱即用的Docker镜像
- 📦 **多架构** - 支持amd64和arm64架构
- 🔁 **GitHub Actions** - 自动构建和发布镜像

## 快速开始

### 1. 创建配置文件

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```env
# Cloudflare API Token
CLOUDFLARE_API_TOKEN=你的API_Token

# 你的域名
DOMAIN=example.com

# 子域名（根域名使用 @）
SUBDOMAIN=ddns

# 检查间隔（分钟）
CHECK_INTERVAL=5
```

### 2. 获取 Cloudflare API Token

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 进入 [API Tokens](https://dash.cloudflare.com/profile/api-tokens)
3. 点击 **Create Token**
4. 使用模板 **Edit zone DNS** 或自定义创建
5. 所需权限：
   - **Zone** → **DNS** → **Edit**
   - **Zone** → **Zone** → **Read**
6. Zone Resources 选择你的域名
7. 创建并复制 Token

### 3. 启动容器

使用 Docker Compose（推荐）：

```bash
docker-compose up -d
```

或使用 Docker CLI：

```bash
docker run -d \
  --name cloudflare-ddns \
  --restart unless-stopped \
  -e CLOUDFLARE_API_TOKEN=你的Token \
  -e DOMAIN=example.com \
  -e SUBDOMAIN=ddns \
  -e CHECK_INTERVAL=5 \
  -e TZ=Asia/Shanghai \
  -v ddns-data:/data \
  ghcr.io/krystalqaq/ddns:latest
```

### 4. 查看日志

```bash
docker logs -f cloudflare-ddns
```

输出示例：

```
🚀 Cloudflare DDNS 启动
========================================

📋 配置信息:
   域名: ddns.example.com
   检查间隔: 5 分钟

[2026-01-31 14:30:00] 🔍 正在获取Zone ID...
[2026-01-31 14:30:01] ✅ Zone ID: abc1234567890

[2026-01-31 14:30:01] 🔍 正在获取当前公网IP...
[2026-01-31 14:30:02] 📍 当前公网IP: 123.45.67.89
[2026-01-31 14:30:02] 🔄 检测到IP变化: (首次运行) -> 123.45.67.89
[2026-01-31 14:30:02] 🔄 正在更新Cloudflare DNS记录...
[2026-01-31 14:30:03] 📝 DNS记录ID: xyz9876543210
[2026-01-31 14:30:03] 📝 原DNS IP: 123.45.67.89
[2026-01-31 14:30:04] ✅ DNS记录更新成功!
[2026-01-31 14:30:04] ✅ ddns.example.com -> 123.45.67.89

[2026-01-31 14:30:04] ⏰ 等待 5 分钟后进行下次检查...
```

## 环境变量

| 变量 | 说明 | 示例 | 必填 |
|------|------|------|------|
| `CLOUDFLARE_API_TOKEN` | Cloudflare API Token | your_token_here | 是 |
| `DOMAIN` | 你的域名 | example.com | 是 |
| `SUBDOMAIN` | 子域名或@ | ddns 或 @ | 否（默认ddns） |
| `CHECK_INTERVAL` | 检查间隔（分钟） | 5 | 否（默认5） |
| `TZ` | 时区 | Asia/Shanghai | 否（默认UTC） |

## 本地开发

### 编译

```bash
# 编译当前平台
go build -o ddns main.go

# 交叉编译
GOOS=linux GOARCH=amd64 go build -o ddns-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -o ddns-linux-arm64 main.go
```

### 本地运行

```bash
# 从环境变量读取
export CLOUDFLARE_API_TOKEN=your_token
export DOMAIN=example.com
export SUBDOMAIN=ddns

./ddns
```

### 构建Docker镜像

```bash
# 构建本地镜像
docker build -t ddns:local .

# 查看镜像大小
docker images ddns:local
```

## GitHub Actions 自动构建

项目包含GitHub Actions工作流，会在以下情况自动构建Docker镜像：

- 推送到main分支
- 创建tag（如v1.0.0）
- 手动触发workflow

### 配置GitHub Container Registry

1. 在GitHub仓库设置中启用Packages
2. 确保仓库有写入Packages的权限
3. 推送代码后自动开始构建

### 使用构建的镜像

修改 `docker-compose.yml` 中的镜像地址：

```yaml
image: ghcr.io/krystalqaq/ddns:latest
```

或直接拉取镜像：

```bash
docker pull ghcr.io/krystalqaq/ddns:latest
```

## 多架构支持

GitHub Actions会自动构建以下架构的镜像：

- linux/amd64
- linux/arm64

Docker会自动拉取对应架构的镜像。

## 故障排除

### 1. API Token 错误

确保：
- Token已正确设置
- Token有正确的权限（DNS Edit + Zone Read）
- Token的Zone Resources包含你的域名

### 2. 找不到DNS记录

确保：
- 域名已添加到Cloudflare
- A记录已存在（脚本不会自动创建）
- DOMAIN和SUBDOMAIN配置正确

### 3. 容器立即退出

查看日志：

```bash
docker logs cloudflare-ddns
```

常见原因：
- 环境变量未设置或设置错误
- 网络无法访问Cloudflare API

### 4. IP未更新

检查：
- 容器是否有网络访问权限
- `/data/current_ip.txt` 中的IP是否正确

```bash
docker exec cloudflare-ddns cat /data/current_ip.txt
```

## 镜像大小优化

本项目使用了多种优化技术：

1. **多阶段构建** - 使用alpine镜像编译，scratch镜像运行
2. **静态编译** - CGO_ENABLED=0 生成静态二进制
3. **去除调试信息** - -ldflags="-s -w" 减小文件大小
4. **scratch基础镜像** - 不包含任何额外文件

最终镜像大小：**约2-3MB**

## 系统要求

- Docker 20.10+
- Docker Compose 1.29+（可选）

## 注意事项

1. 只支持A记录（IPv4）
2. DNS记录必须提前在Cloudflare创建
3. 建议检查间隔不要太短（最少5分钟）
4. 数据目录挂载到 `/data` 用于保存IP缓存

## 项目结构

```
.
├── main.go                 # 主程序
├── go.mod                  # Go模块依赖
├── Dockerfile              # Docker镜像构建文件
├── docker-compose.yml      # Docker Compose配置
├── .env.example            # 环境变量示例
├── .github/
│   └── workflows/
│       └── docker-build.yml # GitHub Actions工作流
└── README.md               # 项目文档
```

## 许可证

MIT

## 贡献

欢迎提交Issue和Pull Request！
