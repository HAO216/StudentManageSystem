# 使用官方的 Go 基础镜像
FROM golang:1.21-alpine

# 设置工作目录
WORKDIR /app

# 复制项目的 go.mod 和 go.sum 文件，以便提前下载依赖
COPY go.mod go.sum ./
RUN go mod download

# 复制项目的所有文件到工作目录
COPY . .

# 构建可执行文件
RUN go build -o main .

# 暴露服务端口，这里假设服务监听 8080 端口
EXPOSE 8080

# 定义容器启动时执行的命令
CMD ["./main"]