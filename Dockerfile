# 多阶段构建 - 构建阶段
FROM golang:1.23-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /build

# 复制go mod文件
COPY go.mod go.sum* ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY main.go ./

# 自动检测目标架构
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

# 编译为静态二进制文件
# CGO_ENABLED=0 禁用CGO，生成纯静态二进制
# -ldflags="-s -w" 去除调试信息和符号表，减小文件大小
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w" \
    -a -installsuffix cgo \
    -o ddns .

# 运行阶段 - 使用最小的基础镜像
FROM scratch

# 从builder阶段复制时区数据和CA证书
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# 从builder阶段复制编译好的二进制文件
COPY --from=builder /build/ddns /ddns

# 创建数据目录
VOLUME ["/data"]

# 设置环境变量
ENV TZ=Asia/Shanghai

# 运行程序
ENTRYPOINT ["/ddns"]
